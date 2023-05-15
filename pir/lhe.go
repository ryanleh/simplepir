package pir

import (
        "github.com/henrycg/simplepir/rand"
        "github.com/henrycg/simplepir/matrix"
)

type SecretLHE[T matrix.Elem] struct {
	query  *matrix.Matrix[T]
	secret *matrix.Matrix[T]
	arr    *matrix.Matrix[T]
}

func (s *SecretLHE[T]) Copy() *SecretLHE[T] {
	out := new(SecretLHE[T])

	out.query = s.query.Copy()
	out.secret = s.secret.Copy()
	out.arr = s.arr.Copy()

	return out
}

func (c *Client[T]) PreprocessQueryLHE() *SecretLHE[T] {
	if (c.dbinfo.Ne != 1) || ((1 << c.dbinfo.RowLength) > c.params.P) {
		panic("Not yet supported.")
	}

	// checks that p is a power of 2 (since q must be)
	if (c.params.P & (c.params.P - 1)) != 0 {
		panic("LHE requires p | q.")
	}

	//log.Printf("N=%v,  P=%v, L=%v, M=%v", c.dbinfo.Num, c.dbinfo.P(), c.dbinfo.L, c.dbinfo.M)

	s := &SecretLHE[T]{
		secret: matrix.Rand[T](c.prg, c.params.N, 1, 0),
	}

        src := make([]matrix.IoRandSource, len(c.matrixAseeds))
        for i, seed := range c.matrixAseeds {
                src[i] = rand.NewBufPRG(rand.NewPRG(seed))
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

func (c *Client[T]) QueryLHEPreprocessed(arrIn *matrix.Matrix[T], s *SecretLHE[T]) *Query[T] {
	arr := arrIn.Copy()

	if arr.Rows() != c.dbinfo.M || arr.Cols() != 1 {
		panic("Parameter mismatch")
	}

	s.arr = arr
	arr.MulConst(T(c.params.Delta))
	s.query.AddWithMismatch(arr)

	return &Query[T] { s.query.Copy() }
}

func (c *Client[T]) QueryLHE(arrIn *matrix.Matrix[T]) (*SecretLHE[T], *Query[T]) {
	s := c.PreprocessQueryLHE()
	q := c.QueryLHEPreprocessed(arrIn, s)

	return s, q
}

func (c *Client[T]) RecoverManyLHE(secret *SecretLHE[T], ansIn *Answer[T]) *matrix.Matrix[T] {
	if (c.dbinfo.Ne != 1) {
		panic("Not yet supported")
	}
  
  ans := ansIn.answer.Copy()


	interm := matrix.Mul(c.hint, secret.secret)
	ans.Sub(interm)

	out := matrix.Zeros[T](ans.Rows(), 1)
	for row := uint64(0); row < ans.Rows(); row++ {
		noised := uint64(ans.Get(row, 0))
    //log.Printf("noised[%v] = %v   [Delta=%v]", row, noised, c.params.Delta)
		denoised := c.params.Round(noised)
		out.Set(row, 0, T(denoised % c.params.P))
	}

	return out
}
