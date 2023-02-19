package pir

import (
//  "log"
  "github.com/henrycg/simplepir/matrix"
)

type SecretLHE[T matrix.Elem] struct {
	query  *matrix.Matrix[T]
	secret *matrix.Matrix[T]
	arr    *matrix.Matrix[T]
}

func (c *Client[T]) QueryLHE(arrIn *matrix.Matrix[T]) (*SecretLHE[T], *Query[T]) {
  arr := arrIn.Copy()

	if arr.Rows() != c.dbinfo.M || arr.Cols() != 1 {
		panic("Parameter mismatch")
	}

	if (c.dbinfo.Packing != 1) || (c.dbinfo.Ne != 1) || ((1 << c.dbinfo.RowLength) > c.params.P) {
		panic("Not yet supported.")
	}

	// checks that p is a power of 2 (since q must be)
	if (c.params.P & (c.params.P - 1)) != 0 {
		panic("LHE requires p | q.")
	}

  //log.Printf("N=%v,  P=%v, L=%v, M=%v", c.dbinfo.Num, c.dbinfo.P(), c.dbinfo.L, c.dbinfo.M)

	s := &SecretLHE[T]{
		secret: matrix.Rand[T](c.prg, c.params.N, 1, 0),
		arr:    arr,
	}

	err := matrix.Gaussian[T](c.prg, c.dbinfo.M, 1)

	query := matrix.Mul(c.matrixA, s.secret)
	query.Add(err)

  arr.MulConst(T(c.params.Delta))
  query.Add(arr)

	// Pad the query to match the dimensions of the compressed DB
	if c.dbinfo.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.dbinfo.M % c.dbinfo.Squishing))
	}

	s.query = query.Copy()

	return s, &Query[T]{ query }
}

func (c *Client[T]) RecoverManyLHE(secret *SecretLHE[T], ans *Answer[T]) *matrix.Matrix[T] {
	if (c.dbinfo.Packing != 1) || (c.dbinfo.Ne != 1) {
		panic("Not yet supported")
	}

	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.dbinfo.M; j++ {
		offset += ratio * uint64(secret.query.Get(j, 0))
	}
	offset = -offset

  if T(0).Bitlen() == 32 {
    offset %= (1 << 32)
  }
  //log.Printf("offset=%v", offset)

	interm := matrix.Mul(c.hint, secret.secret)
  acopy := ans.answer.Copy()
	acopy.Sub(interm)
	acopy.AddConst(T(offset))

	norm := uint64(0)
  for i := uint64(0); i<secret.arr.Rows(); i++ {
		norm += uint64(secret.arr.Get(i, 0))
	}
	norm %= 2
  //log.Printf("Norm: %v", norm)

	out := matrix.Zeros[T](acopy.Rows(), 1)
	for row := uint64(0); row < acopy.Rows(); row++ {
		noised := uint64(acopy.Get(row, 0))
    //log.Printf("noised[%v] = %v   [Delta=%v]", row, noised, c.params.Delta)
		denoised := c.params.Round(noised)
		out.Set(row, 0, T((denoised + ratio*norm) % c.params.P))
	}

	return out
}
