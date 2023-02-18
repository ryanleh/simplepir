package pir

import "bytes"
import "encoding/gob"
import "github.com/henrycg/simplepir/lwe"
import "github.com/henrycg/simplepir/rand"
import "github.com/henrycg/simplepir/matrix"

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
type SecretLHE[T matrix.Elem] struct {
	query  *matrix.Matrix[T]
	secret *matrix.Matrix[T]
	arr    []uint64
}

type Answer[T matrix.Elem] struct {
  answer *matrix.Matrix[T]
}

func NewServer[T matrix.Elem](db *Database[T]) *Server[T] {
	prg := rand.NewRandomBufPRG()
	params := db.Info.Params
	matrixA := matrix.Rand[T](prg, db.Info.M, params.N, params.Logq, 0)
	return setupServer(db, matrixA)
}

func NewServerSeed[T matrix.Elem](db *Database[T], seed *rand.PRGKey) *Server[T] {
	prg := rand.NewBufPRG(rand.NewPRG(seed))
	params := db.Info.Params
	matrixA := matrix.Rand[T](prg, db.Info.M, params.N, params.Logq, 0)
	return setupServer(db, matrixA)
}

func setupServer[T matrix.Elem](db *Database[T], matrixA *matrix.Matrix[T]) *Server[T] {
	s := &Server[T]{
		params:  db.Info.Params,
		matrixA: matrixA,
		db:      db.Copy(),
		hint:    matrix.Mul(db.Data, matrixA),
	}

	// map the database entries to [0, p] (rather than [-p/1, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	s.db.Data.AddUint64(s.params.P / 2)
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
		secret: matrix.Rand[T](c.prg, c.params.N, 1, c.params.Logq, 0),
		index:  i,
	}

	err := matrix.Gaussian[T](c.prg, c.dbinfo.M, 1)

	query := matrix.Mul(c.matrixA, s.secret)
	query.Add(err)
	query.AddAt(c.params.Delta(), i%c.dbinfo.M, 0)

	// Pad the query to match the dimensions of the compressed DB
	if c.dbinfo.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.dbinfo.M % c.dbinfo.Squishing))
	}

	s.query = query

	return s, &Query[T]{ query }
}

func (c *Client[T]) QueryLHE(arr []uint64) (*SecretLHE[T], *Query[T]) {
	if uint64(len(arr)) != c.dbinfo.M {
		panic("Parameter mismatch")
	}

	if (c.dbinfo.Packing != 1) || (c.dbinfo.Ne != 1) || ((1 << c.dbinfo.RowLength) > c.params.P) {
		panic("Not yet supported.")
	}

	// checks that p is a power of 2 (since q must be)
	if (c.params.P & (c.params.P - 1)) != 0 {
		panic("LHE requires p | q.")
	}

	s := &SecretLHE[T]{
		secret: matrix.Rand[T](c.prg, c.params.N, 1, c.params.Logq, 0),
		arr:    arr,
	}

	err := matrix.Gaussian[T](c.prg, c.dbinfo.M, 1)

	query := matrix.Mul(c.matrixA, s.secret)
	query.Add(err)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		query.AddAt(c.params.Delta()*arr[j], j, 0)
	}

	// Pad the query to match the dimensions of the compressed DB
	if c.dbinfo.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.dbinfo.M % c.dbinfo.Squishing))
	}

	s.query = query.Copy()

	return s, &Query[T]{ query }
}

func (s *Server[T]) Answer(query *Query[T]) *Answer[T] {
	return &Answer[T]{ matrix.MulVecPacked(s.db.Data, query.query) }
}

func (c *Client[T]) Recover(secret *Secret[T], ans *Answer[T]) uint64 {
	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		offset += ratio * secret.query.Get(j, 0)
	}
	offset %= (1 << c.params.Logq)
	offset = (1 << c.params.Logq) - offset

	row := secret.index / c.dbinfo.M
	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := ans.answer.Get(j, 0) + offset
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.answer.Add(interm)

	return c.dbinfo.ReconstructElem(vals, secret.index)
}

func (c *Client[T]) RecoverMany(secret *Secret[T], ans *Answer[T]) []uint64 {
	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		offset += ratio * secret.query.Get(j, 0)
	}
	offset %= (1 << c.params.Logq)
	offset = (1 << c.params.Logq) - offset

	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

	num_rows := ans.answer.Rows() / c.dbinfo.Ne
	i := secret.index % c.dbinfo.M
	out := make([]uint64, num_rows)
	for row := uint64(0); row < num_rows; row++ {
		var vals []uint64
		// Recover each Z_p element that makes up the desired database entry
		for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
			noised := ans.answer.Get(j, 0) + offset
			denoised := c.params.Round(noised)
			vals = append(vals, denoised)
			//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
		}
		out[row] = c.dbinfo.ReconstructElem(vals, i)
		i += c.dbinfo.M
	}
	ans.answer.Add(interm)

	return out
}

func (c *Client[T]) RecoverManyLHE(secret *SecretLHE[T], ans *Answer[T]) []uint64 {
	if (c.dbinfo.Packing != 1) || (c.dbinfo.Ne != 1) {
		panic("Not yet supported")
	}

	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		offset += ratio * secret.query.Get(j, 0)
	}
	offset %= (1 << c.params.Logq)
	offset = (1 << c.params.Logq) - offset

	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

	norm := uint64(0)
	for _, elem := range secret.arr {
		norm += elem
	}
	norm %= 2

	out := make([]uint64, ans.answer.Rows())
	for row := uint64(0); row < ans.answer.Rows(); row++ {
		noised := ans.answer.Get(row, 0) + offset
		denoised := c.params.Round(noised)
		out[row] = (denoised + ratio*norm) % c.params.P
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
