package pir

import (
  "bytes"
  "encoding/gob"
  //"log"
)

import (
  "github.com/henrycg/simplepir/lwe"
  "github.com/henrycg/simplepir/rand"
  "github.com/henrycg/simplepir/matrix"
)

type Server[T matrix.Elem] struct {
	params  *lwe.Params
	matrixA *matrix.Matrix[T]

	db   *Database[T]
	hint *matrix.Matrix[T]
}

type Client[T matrix.Elem] struct {
	prg *rand.BufPRGReader

	params *lwe.Params
	hint   *matrix.Matrix[T]

	matrixA *matrix.Matrix[T]
	dbinfo  *DBInfo
}

type Query[T matrix.Elem] struct {
  query *matrix.Matrix[T]
}

type Secret[T matrix.Elem] struct {
	query  *matrix.Matrix[T]
	secret *matrix.Matrix[T]
	index  uint64
}

type Answer[T matrix.Elem] struct {
  answer *matrix.Matrix[T]
}

func NewServer[T matrix.Elem](db *Database[T]) *Server[T] {
	prg := rand.NewRandomBufPRG()
	params := db.Info.Params
	matrixA := matrix.Rand[T](prg, db.Info.M, params.N, 0)
	return setupServer(db, matrixA)
}

func NewServerSeed[T matrix.Elem](db *Database[T], seed *rand.PRGKey) *Server[T] {
	prg := rand.NewBufPRG(rand.NewPRG(seed))
	params := db.Info.Params
	matrixA := matrix.Rand[T](prg, db.Info.M, params.N, 0)
	return setupServer(db, matrixA)
}

func setupServer[T matrix.Elem](db *Database[T], matrixA *matrix.Matrix[T]) *Server[T] {
	s := &Server[T]{
		params:  db.Info.Params,
		matrixA: matrixA,
		db:      db.Copy(),
		hint:    matrix.Mul(db.Data, matrixA),
	}

	// map the database entries to [0, p] (rather than [-p/2, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	s.db.Data.AddConst(T(s.params.P / 2))
	s.db.Squish()

	return s
}

func (s *Server[T]) Hint() *matrix.Matrix[T] {
	return s.hint
}

func (s *Server[T]) MatrixA() *matrix.Matrix[T] {
	return s.matrixA
}

func (s *Server[T]) Params() *lwe.Params {
	return s.params
}

func (s *Server[T]) Get(i uint64) uint64 {
	return s.db.GetElem(i)
}

func (s *Server[T]) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(s.params)
	if err != nil {
		return buf.Bytes(), err
	}

	err = enc.Encode(s.matrixA) // TODO: Improve by storing just a see
	if err != nil {
		return buf.Bytes(), err
	}

	err = enc.Encode(s.db)
	if err != nil {
		return buf.Bytes(), err
	}

	err = enc.Encode(s.hint)
	return buf.Bytes(), err
}

func (s *Server[T]) GobDecode(buf []byte) error {
	b := bytes.NewBuffer(buf)
	dec := gob.NewDecoder(b)
	err := dec.Decode(&s.params)
	if err != nil {
		return err
	}

	err = dec.Decode(&s.matrixA)
	if err != nil {
		return err
	}

	err = dec.Decode(&s.db)
	if err != nil {
		return err
	}

	err = dec.Decode(&s.hint)
	return err
}

func NewClient[T matrix.Elem](hint *matrix.Matrix[T], matrixA *matrix.Matrix[T], dbinfo *DBInfo) *Client[T] {
	return &Client[T]{
		prg: rand.NewRandomBufPRG(),

		params: dbinfo.Params,
		hint:   hint.Copy(),

		matrixA: matrixA.Copy(),
		dbinfo:  dbinfo,
	}
}

func (c *Client[T]) Query(i uint64) (*Secret[T], *Query[T]) {
	s := &Secret[T]{
		secret: matrix.Rand[T](c.prg, c.params.N, 1, 0),
		//secret: matrix.Zeros[T](c.params.N, 1),
		index:  i,
	}

	err := matrix.Gaussian[T](c.prg, c.dbinfo.M, 1)
	//err := matrix.Zeros[T](c.dbinfo.M, 1)

	query := matrix.Mul(c.matrixA, s.secret)
	query.Add(err)
	query.AddAt(i%c.dbinfo.M, 0, T(c.params.Delta))

	// Pad the query to match the dimensions of the compressed DB
	if c.dbinfo.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.dbinfo.M % c.dbinfo.Squishing))
	}

	s.query = query

	return s, &Query[T]{ query }
}


func (s *Server[T]) Answer(query *Query[T]) *Answer[T] {
	return &Answer[T]{ matrix.MulVecPacked(s.db.Data, query.query) }
}

func (c *Client[T]) Recover(secret *Secret[T], ans *Answer[T]) uint64 {
  if c.dbinfo.Packing > 1 {
    panic("Not supported")
  }

	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		offset += ratio * uint64(secret.query.Get(j, 0))
	}

  offset = -offset
  if T(0).Bitlen() == 32 {
    offset %= (1<<32)
  }

	row := secret.index / c.dbinfo.M
	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := uint64(ans.answer.Get(j, 0)) + offset
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//log.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.answer.Add(interm)

	return c.dbinfo.ReconstructElem(vals, secret.index)
}

func (c *Client[T]) RecoverMany(secret *Secret[T], ans *Answer[T]) []uint64 {
  if c.dbinfo.Packing > 1 {
    panic("Not supported")
  }

	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		offset += ratio * uint64(secret.query.Get(j, 0))
	}
	offset = -offset 
  if T(0).Bitlen() == 32 {
    offset %= (1<<32)
  }

	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

  num_values := (ans.answer.Rows() / c.dbinfo.Ne)
	out := make([]uint64, num_values)
	for row := uint64(0); row < ans.answer.Rows(); row += c.dbinfo.Ne {
		var vals []uint64
		// Recover each Z_p element that makes up the desired database entry
		for j := uint64(0); j < c.dbinfo.Ne; j++ {
			noised := uint64(ans.answer.Get(row + j, 0)) + offset
			denoised := c.params.Round(noised)
			vals = append(vals, denoised)
		}

		for j := uint64(0); j < c.dbinfo.Packing; j++ {
		  out[row] = c.dbinfo.ReconstructElem(vals, j)
    }
		//log.Printf("Reconstructing row %d: %d\n", row, out[row])
	}
	ans.answer.Add(interm)

	return out
}


func (c *Client[T]) GetM() uint64 {
	return c.dbinfo.M
}

func (c *Client[T]) GetL() uint64 {
	return c.dbinfo.L
}

func (c *Client[T]) GetP() uint64 {
	return c.params.P
}
