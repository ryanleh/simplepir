package matrix

// #cgo CFLAGS: -O3 -march=native
// #include "matrix.h"
import "C"

import (
  "fmt"
	"unsafe"
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

func (a *Matrix[T]) AddUint64(val uint64) {
	v := T(val)
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] += v
	}
}

func (a *Matrix[T]) AddAt(val, i, j uint64) {
	if (i >= a.rows) || (j >= a.cols) {
		panic("Out of bounds")
	}
	a.Set(a.Get(i, j)+val, i, j)
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

func (a *Matrix[T]) SubUint64(val uint64) {
	v := T(val)
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] -= v
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

  if a.Is32Bit() {
    C.matMul32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols, bcols)
  } else if a.Is64Bit() {
    C.matMul64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols, bcols)
  } else {
    panic("Shouldn't get here")
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

  if a.Is32Bit() {
      C.matMulVec32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols)
  } else if a.Is64Bit() {
      C.matMulVec64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols)
  } else {
    panic("Shouldn't get here")
  }

	return out
}

func MulVecPacked[T Elem](a *Matrix[T], b *Matrix[T]) *Matrix[T] {
	if a.cols*a.SquishRatio() != b.rows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
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

  if a.Is32Bit() {
      C.matMulVecPacked32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols)
  } else if a.Is64Bit() {
      C.matMulVecPacked64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols)
  } else {
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

