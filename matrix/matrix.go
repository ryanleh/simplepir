package matrix

// #cgo CFLAGS: -O3 -march=native
// #include "matrix.h"
import "C"

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"io"
	"math/big"
	mrand "math/rand"
	"reflect"
	"unsafe"

  "github.com/henrycg/simplepir/lwe"
)

var t32 = reflect.TypeOf(Elem32(0))
var t64 = reflect.TypeOf(Elem64(0))

type Elem32 = C.Elem32
type Elem64 = C.Elem64

type elem interface {
    Elem32 | Elem64
}

const SquishBasis32 = C.BASIS_32
const SquishRatio32 = C.COMPRESSION_32

type IoRandSource interface {
	io.Reader
	mrand.Source64
}

type Matrix[T elem] struct {
	rows uint64
	cols uint64
	data []T
}

func (m *Matrix[T]) Copy() *Matrix[T] {
	out := &Matrix[T]{
		rows: m.rows,
		cols: m.cols,
		data: make([]T, len(m.data)),
	}

	copy(out.data[:], m.data[:])
	return out
}

func (m *Matrix[T]) Rows() uint64 {
	return m.rows
}

func (m *Matrix[T]) Cols() uint64 {
	return m.cols
}

func (m *Matrix[T]) Size() uint64 {
	return m.rows * m.cols
}

func (m *Matrix[T]) AppendZeros(n uint64) {
	m.Concat(Zeros[T](n, 1))
}

func New[T elem](rows uint64, cols uint64) *Matrix[T] {
	out := new(Matrix[T])
	out.rows = rows
	out.cols = cols
	out.data = make([]T, rows*cols)
	return out
}

func Rand[T elem](src IoRandSource, rows uint64, cols uint64, logmod uint64, mod uint64) *Matrix[T] {
	out := New[T](rows, cols)
	m := big.NewInt(int64(mod))
	if mod == 0 {
		m = big.NewInt(1 << logmod)
	}
	for i := 0; i < len(out.data); i++ {
		v, err := rand.Int(src, m)
		if err != nil {
			panic("Randomness error")
		}
		out.data[i] = T(v.Uint64())
	}
	return out
}

func Zeros[T elem](rows uint64, cols uint64) *Matrix[T] {
	out := New[T](rows, cols)
	for i := 0; i < len(out.data); i++ {
		out.data[i] = T(0)
	}
	return out
}

func (m *Matrix[T]) ReduceMod(p uint64) {
	mod := T(p)
	for i := 0; i < len(m.data); i++ {
		m.data[i] = m.data[i] % mod
	}
}

func (m *Matrix[T]) Get(i, j uint64) uint64 {
	if i >= m.rows {
		panic("Too many rows!")
	}
	if j >= m.cols {
		panic("Too many cols!")
	}
	return uint64(m.data[i*m.cols+j])
}

func (m *Matrix[T]) Set(val, i, j uint64) {
	if i >= m.rows {
		panic("Too many rows!")
	}
	if j >= m.cols {
		panic("Too many cols!")
	}
	m.data[i*m.cols+j] = T(val)
}

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

func Mul[T elem](a *Matrix[T], b *Matrix[T]) *Matrix[T] {
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

  switch tin := reflect.TypeOf(a.data[0]); tin {
    case t32:
      C.matMul32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols, bcols)
    case t64:
      C.matMul64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols, bcols)
    default:
      panic("Shouldn't get here")
  }

  return out
}


func MulVec[T elem](a *Matrix[T], b *Matrix[T]) *Matrix[T] {
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

  switch tin := reflect.TypeOf(a.data[0]); tin {
    case t32:
      C.matMulVec32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols)
    case t64:
      C.matMulVec64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols)
    default:
      panic("Shouldn't get here")
  }

	return out
}

func MulVecPacked[T elem](a *Matrix[T], b *Matrix[T], basis, compression uint64) *Matrix[T] {
	if a.cols*compression != b.rows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	if b.cols != 1 {
		panic("Second argument is not a vector")
	}
	if compression != 3 && basis != 10 {
		panic("Must use hard-coded values!")
	}

	out := New[T](a.rows+8, 1)
	arows := C.size_t(a.rows)
	acols := C.size_t(a.cols)

  outPtr := unsafe.Pointer(&out.data[0])
  aPtr := unsafe.Pointer(&a.data[0])
  bPtr := unsafe.Pointer(&b.data[0])

  switch tin := reflect.TypeOf(a.data[0]); tin {
    case t32:
      C.matMulVecPacked32((*Elem32)(outPtr), (*Elem32)(aPtr), (*Elem32)(bPtr), arows, acols)
    case t64:
      C.matMulVecPacked64((*Elem64)(outPtr), (*Elem64)(aPtr), (*Elem64)(bPtr), arows, acols)
    default:
      panic("Shouldn't get here")
  }
	out.DropLastrows(8)

	return out
}

func (a *Matrix[T]) Concat(b *Matrix[T]) {
	if a.cols == 0 && a.rows == 0 {
		a.cols = b.cols
		a.rows = b.rows
		a.data = b.data
		return
	}

	if a.cols != b.cols {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}

	a.rows += b.rows
	a.data = append(a.data, b.data...)
}

// Compresses the matrix to store it in 'packed' form.
// Specifically, this method squishes the matrix by representing each
// group of 'delta' consecutive values as a single database element,
// where each value uses 'basis' bits.
func (m *Matrix[T]) Squish(basis, delta uint64) {
	n := Zeros[T](m.rows, (m.cols+delta-1)/delta)

	for i := uint64(0); i < n.rows; i++ {
		for j := uint64(0); j < n.cols; j++ {
			for k := uint64(0); k < delta; k++ {
				if delta*j+k < m.cols {
					val := m.Get(i, delta*j+k)
					n.data[i*n.cols+j] += T(val << (k * basis))
				}
			}
		}
	}

	m.cols = n.cols
	m.rows = n.rows
	m.data = n.data
}

func (m *Matrix[T]) Round(round_to uint64, mod uint64) {
	for i := uint64(0); i < m.rows*m.cols; i++ {
		v := (uint64(m.data[i]) + round_to/2) / round_to
		m.data[i] = T(v % mod)
	}
}

func (m *Matrix[T]) DropLastrows(n uint64) {
	m.rows -= n
	m.data = m.data[:(m.rows * m.cols)]
}

func (m *Matrix[T]) GetRow(offset, num_rows uint64) *Matrix[T] {
	if offset+num_rows > m.rows {
		panic("Requesting too many rows")
	}

	m2 := New[T](num_rows, m.cols)
	m2.data = m.data[(offset * m.cols):((offset + num_rows) * m.cols)]
	return m2
}

func (m *Matrix[T]) rowsDeepCopy(offset, num_rows uint64) *Matrix[T] {
	if offset+num_rows > m.rows {
		panic("Requesting too many rows")
	}

	if offset+num_rows <= m.rows {
		m2 := New[T](num_rows, m.cols)
		copy(m2.data, m.data[(offset*m.cols):((offset+num_rows)*m.cols)])
		return m2
	}

	m2 := New[T](m.rows-offset, m.cols)
	copy(m2.data, m.data[(offset*m.cols):(m.rows)*m.cols])
	return m2
}

func (m *Matrix[T]) Dim() {
	fmt.Printf("Dims: %d-by-%d\n", m.rows, m.cols)
}

func (m *Matrix[T]) Print() {
	fmt.Printf("%d-by-%d matrix:\n", m.rows, m.cols)
	for i := uint64(0); i < m.rows; i++ {
		for j := uint64(0); j < m.cols; j++ {
			fmt.Printf("%d ", m.data[i*m.cols+j])
		}
		fmt.Printf("\n")
	}
}

func (m *Matrix[T]) PrintStart() {
	fmt.Printf("%d-by-%d matrix:\n", m.rows, m.cols)
	for i := uint64(0); i < 2; i++ {
		for j := uint64(0); j < 2; j++ {
			fmt.Printf("%d ", m.data[i*m.cols+j])
		}
		fmt.Printf("\n")
	}
}

func (m *Matrix[T]) Equals(n *Matrix[T]) bool {
	if m.Cols() != n.Cols() {
		return false
	}
	if m.Rows() != n.Rows() {
		return false
	}
	return reflect.DeepEqual(m.data, n.data)
}

func (m Matrix[T]) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err1 := encoder.Encode(m.rows)
	err2 := encoder.Encode(m.cols)
	err3 := encoder.Encode(m.data)

	if err1 != nil || err2 != nil || err3 != nil {
		panic("Gob encoding error")
	}

	return buf.Bytes(), nil
}

func (m *Matrix[T]) GobDecode(buf []byte) error {
	b := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(b)
	err1 := decoder.Decode(&m.rows)
	err2 := decoder.Decode(&m.cols)

	m.data = make([]T, m.rows*m.cols)
	err3 := decoder.Decode(&m.data)

	if err1 != nil || err2 != nil || err3 != nil {
		panic("Gob decoding error")
	}

	return nil
}

func Gaussian[T elem](src IoRandSource, rows, cols uint64) *Matrix[T] {
	out := New[T](rows, cols)
  samplef := lwe.GaussSample32
  switch tin := reflect.TypeOf(T(0)); tin {
    case t32:
      // Do nothing
    case t64:
      samplef = lwe.GaussSample64
    default:
      panic("Shouldn't get here")
  }

	for i := 0; i < len(out.data); i++ {
		out.data[i] = T(samplef(src))
	}
	return out
}

