package pir

import (
	"github.com/henrycg/simplepir/lwe"
	"github.com/henrycg/simplepir/rand"
	"github.com/henrycg/simplepir/matrix"
)

type Client[T matrix.Elem] struct {
	prg          *rand.BufPRGReader

	params       *lwe.Params
	dbinfo       *DBInfo
	hint         *matrix.Matrix[T]

	matrixAseeds []rand.PRGKey
	matrixArows  []uint64
}

func NewClient[T matrix.Elem](hint *matrix.Matrix[T], matrixAseed *rand.PRGKey, dbinfo *DBInfo) *Client[T] {
	return NewClientDistributed(hint, []rand.PRGKey { *matrixAseed}, []uint64{ dbinfo.M }, dbinfo)
}

func NewClientDistributed[T matrix.Elem](hint *matrix.Matrix[T], matrixAseeds []rand.PRGKey, matrixArows []uint64, dbinfo *DBInfo) *Client[T] {
  c := &Client[T]{
		prg: rand.NewRandomBufPRG(),

		params: dbinfo.Params,
		dbinfo:  dbinfo,

		matrixAseeds: matrixAseeds, // Warning: not copied
		matrixArows: matrixArows,   // Warning: not copied
	}

  if hint != nil {
		c.hint = hint.Copy()
  }

  return c
}

func (c *Client[T]) PreprocessQuery() *Secret[T] {
	s := &Secret[T]{
		secret: matrix.Gaussian[T](c.prg, c.params.N, 1),
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

func (c *Client[T]) Recover(secret *Secret[T], ans *Answer[T]) uint64 {
	row := secret.index / c.dbinfo.M
	ans.Answer.Sub(secret.interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := uint64(ans.Answer.Get(j, 0))
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//log.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.Answer.Add(secret.interm)

	return c.dbinfo.ReconstructElem(vals, secret.index)
}

func (c *Client[T]) RecoverMany(secret *Secret[T], ansIn *Answer[T]) []uint64 {
	ans := ansIn.Answer.Copy()
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

func (c *Client[T]) ClearHint() {
	c.hint = nil
}
