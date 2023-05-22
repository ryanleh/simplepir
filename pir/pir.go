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
	params       *lwe.Params
	matrixAseed  *rand.PRGKey

	db           *Database[T]
	hint         *matrix.Matrix[T]
}

type Client[T matrix.Elem] struct {
	prg          *rand.BufPRGReader

	params       *lwe.Params
	dbinfo       *DBInfo
	hint         *matrix.Matrix[T]

	matrixAseeds []rand.PRGKey
	matrixArows  []uint64
}

type Query[T matrix.Elem] struct {
	query *matrix.Matrix[T]
}

type Secret[T matrix.Elem] struct {
	query  *matrix.Matrix[T]
	secret *matrix.Matrix[T]
	interm *matrix.Matrix[T]
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

	out.matrixAseeds = c.matrixAseeds // Warning: not copied
	out.matrixArows = c.matrixArows // Warning: not copied

	out.hint = c.hint.Copy()

	return out
}

func (q *Query[T]) Dim() (uint64, uint64) {
	return q.query.Rows(), q.query.Cols()
}

func (a *Answer[T]) Dim() (uint64, uint64) {
	return a.answer.Rows(), a.answer.Cols()
}

func (q *Query[T]) AppendZeros(num uint64) {
        q.query.AppendZeros(num)
}

func (q *Query[T]) SelectRows(start, num, squishing uint64) *Query[T] {
	res := new(Query[T])
	res.query = q.query.RowsDeepCopy(start, num)

	r, c := res.Dim()
	if (r * c) % squishing != 0 {
		res.AppendZeros(squishing - ((r * c) % squishing)) 
	}

	return res
}

func (a1 *Answer[T]) Add(a2 *Answer[T]) {
	a1.answer.Add(a2.answer)
}

func (a1 *Answer[T]) AddWithMismatch(a2 *Answer[T]) {
	a1.answer.AddWithMismatch(a2.answer)
}

func NewServer[T matrix.Elem](db *Database[T]) *Server[T] {
	return setupServer(db, rand.RandomPRGKey())
}

func NewServerSeed[T matrix.Elem](db *Database[T], seed *rand.PRGKey) *Server[T] {
	return setupServer(db, seed)
}

func setupServer[T matrix.Elem](db *Database[T], matrixAseed *rand.PRGKey) *Server[T] {
	src := rand.NewBufPRG(rand.NewPRG(matrixAseed))
	//matrixAseeded := matrix.NewSeeded[T]([]matrix.IoRandSource{ src }, []uint64{ db.Info.M }, db.Info.Params.N)
        matrixA := matrix.Rand[T](src, db.Info.M, db.Info.Params.N, 0)

	s := &Server[T]{
		params:      db.Info.Params,
		matrixAseed: matrixAseed,
		db:          db.Copy(),
		//hint:        matrix.MulSeededRight(db.Data, matrixAseeded),
		hint:        matrix.Mul(db.Data, matrixA),
	}

	s.db.Squish()

	return s
}

func (s *Server[T]) Hint() *matrix.Matrix[T] {
	return s.hint
}

func (s *Server[T]) MatrixA() *rand.PRGKey {
	return s.matrixAseed
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

func NewClient[T matrix.Elem](hint *matrix.Matrix[T], matrixAseed *rand.PRGKey, dbinfo *DBInfo) *Client[T] {
	return NewClientDistributed(hint, []rand.PRGKey { *matrixAseed}, []uint64{ dbinfo.M }, dbinfo)
}

func NewClientDistributed[T matrix.Elem](hint *matrix.Matrix[T], matrixAseeds []rand.PRGKey, matrixArows []uint64, dbinfo *DBInfo) *Client[T] {
	return &Client[T]{
		prg: rand.NewRandomBufPRG(),

		params: dbinfo.Params,
		hint:   hint.Copy(),
		dbinfo:  dbinfo,

		matrixAseeds: matrixAseeds, // Warning: not copied
		matrixArows: matrixArows,   // Warning: not copied
	}
}

func (c *Client[T]) PreprocessQuery() *Secret[T] {
	s := &Secret[T]{
		secret: matrix.Rand[T](c.prg, c.params.N, 1, 0),
	}

	s.interm = matrix.Mul(c.hint, s.secret)

	src := make([]matrix.IoRandSource, len(c.matrixAseeds))
	for i, seed := range c.matrixAseeds {
		src[i] = rand.NewBufPRG(rand.NewPRG(&seed))
	}
        matrixAseeded := matrix.NewSeeded[T](src, c.matrixArows, c.params.N)

	err := matrix.Gaussian[T](c.prg, c.dbinfo.M, 1)

	query := matrix.MulSeededLeft(matrixAseeded, s.secret)
	query.Add(err)

	// Pad the query to match the dimensions of the compressed DB
	if c.dbinfo.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.dbinfo.M % c.dbinfo.Squishing))
	}

	s.query = query

	return s
}

func (c *Client[T]) QueryPreprocessed(i uint64, s *Secret[T]) *Query[T] {
	s.index = i
	s.query.AddAt(i%c.dbinfo.M, 0, T(c.params.Delta))
	return &Query[T]{ s.query.Copy() }
}

func (c *Client[T]) Query(i uint64) (*Secret[T], *Query[T]) {
	s := c.PreprocessQuery()
	q := c.QueryPreprocessed(i, s)
	return s, q
}


func (s *Server[T]) Answer(query *Query[T]) *Answer[T] {
	return &Answer[T]{ matrix.MulVecPacked(s.db.Data, query.query) }
}

func (c *Client[T]) Recover(secret *Secret[T], ans *Answer[T]) uint64 {
	row := secret.index / c.dbinfo.M
	ans.answer.Sub(secret.interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := uint64(ans.answer.Get(j, 0))
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//log.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.answer.Add(secret.interm)

	return c.dbinfo.ReconstructElem(vals, secret.index)
}

func (c *Client[T]) RecoverMany(secret *Secret[T], ansIn *Answer[T]) []uint64 {
	ans := ansIn.answer.Copy()
	ans.Sub(secret.interm)

	num_values := (ans.Rows() / c.dbinfo.Ne)
	out := make([]uint64, num_values)
	for row := uint64(0); row < ans.Rows(); row += c.dbinfo.Ne {
		var vals []uint64
		// Recover each Z_p element that makes up the desired database entry
		for j := uint64(0); j < c.dbinfo.Ne; j++ {
			noised := uint64(ans.Get(row + j, 0))
			denoised := c.params.Round(noised)
			vals = append(vals, denoised)
		}

		out[row] = c.dbinfo.ReconstructElem(vals, 0)
		//log.Printf("Reconstructing row %d: %d\n", row, out[row])
	}

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
