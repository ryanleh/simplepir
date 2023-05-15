package matrix

// #cgo CFLAGS: -O3 -march=native
// #include "matrix.h"
import "C"

import (
	"crypto/rand"
	"fmt"
	"unsafe"
	"math/big"
)

func (a *Matrix[T]) Add(b *Matrix[T]) {
	if (a.cols != b.cols) || (a.rows != b.rows) {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] += b.data[i]
	}
}

func (a *Matrix[T]) AddWithMismatch(b *Matrix[T]) {
	if a.cols != b.cols {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}

	if a.rows < b.rows {
		a.Concat(Zeros[T](b.rows-a.rows, a.cols))
	}

	for i := uint64(0); i < b.cols*b.rows; i++ {
		a.data[i] += b.data[i]
	}
}

func (a *Matrix[T]) MulConst(val T) {
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] *= val
	}
}

func (a *Matrix[T]) ModConst(val T) {
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] %= val
	}
}

func (a *Matrix[T]) AddConst(val T) {
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] += val
	}
}

func (a *Matrix[T]) AddAt(i, j uint64, val T) {
	if (i >= a.rows) || (j >= a.cols) {
		panic("Out of bounds")
	}
	a.Set(i, j, a.Get(i, j)+val)
}

func (a *Matrix[T]) Sub(b *Matrix[T]) {
	if (a.cols != b.cols) || (a.rows != b.rows) {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] -= b.data[i]
	}
}

func (a *Matrix[T]) SubConst(val T) {
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] -= val
	}
}

func Mul[T Elem](a *Matrix[T], b *Matrix[T]) *Matrix[T] {
	if b.cols == 1 {
		return MulVec(a, b)
	}
	if a.cols != b.rows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}

	out := Zeros[T](a.rows, b.cols)

	arows := C.size_t(a.rows)
	acols := C.size_t(a.cols)
	bcols := C.size_t(b.cols)

	outPtr := unsafe.Pointer(&out.data[0])
	aPtr := unsafe.Pointer(&a.data[0])
	bPtr := unsafe.Pointer(&b.data[0])

	switch T(0).Bitlen() {
		case 32:
			C.matMul32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols, bcols)
		case 64:
			C.matMul64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols, bcols)
		default:
			panic("Shouldn't get here")
	}

	return out
}

func MulSeededLeft[T Elem](aSrc IoRandSource, aRows, aCols, aMod uint64, b *Matrix[T]) *Matrix[T] {
	if aCols != b.rows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", aRows, aCols, b.rows, b.cols)
		panic("Dimension mismatch")
	}

	out := Zeros[T](aRows, b.cols)
	m := big.NewInt(int64(aMod))
	if aMod == 0 {
		m.SetInt64(1)
		m.Lsh(m, uint(T(0).Bitlen()))
	}

	for i := uint64(0); i < aRows; i++ {
		for j := uint64(0); j < aCols; j++ {
			v, err := rand.Int(aSrc, m)
			if err != nil {
				panic("Randomness error")
			}
			val := T(v.Uint64())

			for k := uint64(0); k < b.cols; k++ {
				out.data[i * b.cols + k] += val * b.Get(j, k)
			}
		}
	}

	return out
}

func MulSeededRight[T Elem](a *Matrix[T], bSrc IoRandSource, bRows, bCols, bMod uint64) *Matrix[T] {
	if a.cols != bRows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, bRows, bCols)
		panic("Dimension mismatch")
	}

	out := Zeros[T](a.rows, bCols)
	m := big.NewInt(int64(bMod))
	if bMod == 0 {
		m.SetInt64(1)
		m.Lsh(m, uint(T(0).Bitlen()))
	}

	for j := uint64(0); j < a.cols; j++ {
		for k := uint64(0); k < bCols; k++ {
			v, err := rand.Int(bSrc, m)
			if err != nil {
				panic("Randomness error")
			}
			val := T(v.Uint64())

			for i := uint64(0); i < a.rows; i++ {
				out.data[i * bCols + k] += val * a.Get(i, j)
			}
		}
	}

	return out
}

func MulVec[T Elem](a *Matrix[T], b *Matrix[T]) *Matrix[T] {
	if (a.cols != b.rows) && (a.cols+1 != b.rows) && (a.cols+2 != b.rows) { // do not require exact match because of DB compression
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	if b.cols != 1 {
		panic("Second argument is not a vector")
	}

	out := New[T](a.rows, 1)
	arows := C.size_t(a.rows)
	acols := C.size_t(a.cols)

  outPtr := unsafe.Pointer(&out.data[0])
  aPtr := unsafe.Pointer(&a.data[0])
  bPtr := unsafe.Pointer(&b.data[0])

  switch T(0).Bitlen() {
    case 32:
      C.matMulVec32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols)
    case 64:
      C.matMulVec64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols)
    default:
      panic("Shouldn't get here")
  }

	return out
}

func MulVecPacked[T Elem](a *Matrix[T], b *Matrix[T]) *Matrix[T] {
	if a.cols*a.SquishRatio() != b.rows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		fmt.Printf("Want %v == %v", a.cols*a.SquishRatio(), b.rows)
		panic("Dimension mismatch")
	}
	if b.cols != 1 {
		panic("Second argument is not a vector")
	}

	out := New[T](a.rows+8, 1)
	arows := C.size_t(a.rows)
	acols := C.size_t(a.cols)

  outPtr := unsafe.Pointer(&out.data[0])
  aPtr := unsafe.Pointer(&a.data[0])
  bPtr := unsafe.Pointer(&b.data[0])

  switch T(0).Bitlen() {
    case 32:
      C.matMulVecPacked32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols)
    case 64:
      C.matMulVecPacked64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols)
    default:
      panic("Shouldn't get here")
  }

	out.DropLastrows(8)

	return out
}

func (m *Matrix[T]) Round(round_to uint64, mod uint64) {
	for i := uint64(0); i < m.rows*m.cols; i++ {
		v := (uint64(m.data[i]) + round_to/2) / round_to
		m.data[i] = T(v % mod)
	}
}

