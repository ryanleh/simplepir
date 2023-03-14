package pir

import (
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

func (s *Secret[T]) Copy() *Secret[T] {
	out := new(Secret[T])

	out.query = s.query.Copy()
	out.secret = s.secret.Copy()
	out.index = s.index

	return out
}

func (c *Client[T]) Copy() *Client[T] {
	out := new(Client[T])

	out.prg = rand.NewRandomBufPRG()
	out.params = c.params
	out.dbinfo = c.dbinfo

	out.hint = c.hint.Copy()
	out.matrixA = c.matrixA.Copy()

	return out
}

func (q *Query[T]) Dim() (uint64, uint64) {
	return q.query.Rows(), q.query.Cols()
}

func (a *Answer[T]) Dim() (uint64, uint64) {
	return a.answer.Rows(), a.answer.Cols()
}

func (q *Query[T]) SelectRows(start, num uint64) *Query[T] {
	res := new(Query[T])
	res.query = q.query.RowsDeepCopy(start, num)
	return res
}

func (q *Query[T]) AppendZeros(num uint64) {
	q.query.AppendZeros(num)
}

func (a1 *Answer[T]) Add(a2 *Answer[T]) {
	a1.answer.Add(a2.answer)
}

func (a1 *Answer[T]) AddWithMismatch(a2 *Answer[T]) {
	a1.answer.AddWithMismatch(a2.answer)
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

func (s *Server[T]) DBInfo() *DBInfo {
	return s.db.Info
}

func (s *Server[T]) Get(i uint64) uint64 {
	return s.db.GetElem(i)
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
	row := secret.index / c.dbinfo.M
	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := uint64(ans.answer.Get(j, 0))
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//log.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.answer.Add(interm)

	return c.dbinfo.ReconstructElem(vals, secret.index)
}

func (c *Client[T]) RecoverMany(secret *Secret[T], ans *Answer[T]) []uint64 {
	interm := matrix.Mul(c.hint, secret.secret)
	ans.answer.Sub(interm)

  num_values := (ans.answer.Rows() / c.dbinfo.Ne)
	out := make([]uint64, num_values)
	for row := uint64(0); row < ans.answer.Rows(); row += c.dbinfo.Ne {
		var vals []uint64
		// Recover each Z_p element that makes up the desired database entry
		for j := uint64(0); j < c.dbinfo.Ne; j++ {
			noised := uint64(ans.answer.Get(row + j, 0))
			denoised := c.params.Round(noised)
			vals = append(vals, denoised)
		}

    out[row] = c.dbinfo.ReconstructElem(vals, 0)
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
