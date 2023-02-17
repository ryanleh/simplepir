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
)

type Elem = C.Elem

type IoRandSource interface {
	io.Reader
	mrand.Source64
}

type Matrix struct {
	rows uint64
	cols uint64
	data []C.Elem
}

func (m *Matrix) Copy() *Matrix {
	out := &Matrix{
		rows: m.rows,
		cols: m.cols,
		data: make([]C.Elem, len(m.data)),
	}

	copy(out.data[:], m.data[:])
	return out
}

func (m *Matrix) Rows() uint64 {
	return m.rows
}

func (m *Matrix) Cols() uint64 {
	return m.cols
}

func (m *Matrix) Size() uint64 {
	return m.rows * m.cols
}

func (m *Matrix) AppendZeros(n uint64) {
	m.Concat(Zeros(n, 1))
}

func New(rows uint64, cols uint64) *Matrix {
	out := new(Matrix)
	out.rows = rows
	out.cols = cols
	out.data = make([]C.Elem, rows*cols)
	return out
}

func Rand(src IoRandSource, rows uint64, cols uint64, logmod uint64, mod uint64) *Matrix {
	out := New(rows, cols)
	m := big.NewInt(int64(mod))
	if mod == 0 {
		m = big.NewInt(1 << logmod)
	}
	for i := 0; i < len(out.data); i++ {
		v, err := rand.Int(src, m)
		if err != nil {
			panic("Randomness error")
		}
		out.data[i] = C.Elem(v.Uint64())
	}
	return out
}

func Zeros(rows uint64, cols uint64) *Matrix {
	out := New(rows, cols)
	for i := 0; i < len(out.data); i++ {
		out.data[i] = C.Elem(0)
	}
	return out
}

func Gaussian(src IoRandSource, rows, cols uint64) *Matrix {
	out := New(rows, cols)
	for i := 0; i < len(out.data); i++ {
		out.data[i] = C.Elem(GaussSample(src))
	}
	return out
}

func (m *Matrix) ReduceMod(p uint64) {
	mod := C.Elem(p)
	for i := 0; i < len(m.data); i++ {
		m.data[i] = m.data[i] % mod
	}
}

func (m *Matrix) Get(i, j uint64) uint64 {
	if i >= m.rows {
		panic("Too many rows!")
	}
	if j >= m.cols {
		panic("Too many cols!")
	}
	return uint64(m.data[i*m.cols+j])
}

func (m *Matrix) Set(val, i, j uint64) {
	if i >= m.rows {
		panic("Too many rows!")
	}
	if j >= m.cols {
		panic("Too many cols!")
	}
	m.data[i*m.cols+j] = C.Elem(val)
}

func (a *Matrix) Add(b *Matrix) {
	if (a.cols != b.cols) || (a.rows != b.rows) {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] += b.data[i]
	}
}

func (a *Matrix) AddWithMismatch(b *Matrix) {
	if a.cols != b.cols {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}

	if a.rows < b.rows {
		a.Concat(Zeros(b.rows-a.rows, a.cols))
	}

	for i := uint64(0); i < b.cols*b.rows; i++ {
		a.data[i] += b.data[i]
	}
}

func (a *Matrix) AddUint64(val uint64) {
	v := C.Elem(val)
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] += v
	}
}

func (a *Matrix) AddAt(val, i, j uint64) {
	if (i >= a.rows) || (j >= a.cols) {
		panic("Out of bounds")
	}
	a.Set(a.Get(i, j)+val, i, j)
}

func (a *Matrix) Sub(b *Matrix) {
	if (a.cols != b.cols) || (a.rows != b.rows) {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] -= b.data[i]
	}
}

func (a *Matrix) SubUint64(val uint64) {
	v := C.Elem(val)
	for i := uint64(0); i < a.cols*a.rows; i++ {
		a.data[i] -= v
	}
}

func Mul(a *Matrix, b *Matrix) *Matrix {
	if b.cols == 1 {
		return MulVec(a, b)
	}
	if a.cols != b.rows {
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}

	out := Zeros(a.rows, b.cols)

	outPtr := (*C.Elem)(&out.data[0])
	aPtr := (*C.Elem)(&a.data[0])
	bPtr := (*C.Elem)(&b.data[0])
	arows := C.size_t(a.rows)
	acols := C.size_t(a.cols)
	bcols := C.size_t(b.cols)

	C.matMul(outPtr, aPtr, bPtr, arows, acols, bcols)

	return out
}

func MulVec(a *Matrix, b *Matrix) *Matrix {
	if (a.cols != b.rows) && (a.cols+1 != b.rows) && (a.cols+2 != b.rows) { // do not require exact match because of DB compression
		fmt.Printf("%d-by-%d vs. %d-by-%d\n", a.rows, a.cols, b.rows, b.cols)
		panic("Dimension mismatch")
	}
	if b.cols != 1 {
		panic("Second argument is not a vector")
	}

	out := New(a.rows, 1)

	outPtr := (*C.Elem)(&out.data[0])
	aPtr := (*C.Elem)(&a.data[0])
	bPtr := (*C.Elem)(&b.data[0])
	arows := C.size_t(a.rows)
	acols := C.size_t(a.cols)

	C.matMulVec(outPtr, aPtr, bPtr, arows, acols)

	return out
}

func MulVecPacked(a *Matrix, b *Matrix, basis, compression uint64) *Matrix {
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

	out := New(a.rows+8, 1)

	outPtr := (*C.Elem)(&out.data[0])
	aPtr := (*C.Elem)(&a.data[0])
	bPtr := (*C.Elem)(&b.data[0])

	C.matMulVecPacked(outPtr, aPtr, bPtr, C.size_t(a.rows), C.size_t(a.cols))
	out.DropLastrows(8)

	return out
}

func (a *Matrix) Concat(b *Matrix) {
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
func (m *Matrix) Squish(basis, delta uint64) {
	n := Zeros(m.rows, (m.cols+delta-1)/delta)

	for i := uint64(0); i < n.rows; i++ {
		for j := uint64(0); j < n.cols; j++ {
			for k := uint64(0); k < delta; k++ {
				if delta*j+k < m.cols {
					val := m.Get(i, delta*j+k)
					n.data[i*n.cols+j] += C.Elem(val << (k * basis))
				}
			}
		}
	}

	m.cols = n.cols
	m.rows = n.rows
	m.data = n.data
}

func (m *Matrix) Round(round_to uint64, mod uint64) {
	for i := uint64(0); i < m.rows*m.cols; i++ {
		v := (uint64(m.data[i]) + round_to/2) / round_to
		m.data[i] = C.Elem(v % mod)
	}
}

func (m *Matrix) DropLastrows(n uint64) {
	m.rows -= n
	m.data = m.data[:(m.rows * m.cols)]
}

func (m *Matrix) GetRow(offset, num_rows uint64) *Matrix {
	if offset+num_rows > m.rows {
		panic("Requesting too many rows")
	}

	m2 := New(num_rows, m.cols)
	m2.data = m.data[(offset * m.cols):((offset + num_rows) * m.cols)]
	return m2
}

func (m *Matrix) rowsDeepCopy(offset, num_rows uint64) *Matrix {
	if offset+num_rows > m.rows {
		panic("Requesting too many rows")
	}

	if offset+num_rows <= m.rows {
		m2 := New(num_rows, m.cols)
		copy(m2.data, m.data[(offset*m.cols):((offset+num_rows)*m.cols)])
		return m2
	}

	m2 := New(m.rows-offset, m.cols)
	copy(m2.data, m.data[(offset*m.cols):(m.rows)*m.cols])
	return m2
}

func (m *Matrix) Dim() {
	fmt.Printf("Dims: %d-by-%d\n", m.rows, m.cols)
}

func (m *Matrix) Print() {
	fmt.Printf("%d-by-%d matrix:\n", m.rows, m.cols)
	for i := uint64(0); i < m.rows; i++ {
		for j := uint64(0); j < m.cols; j++ {
			fmt.Printf("%d ", m.data[i*m.cols+j])
		}
		fmt.Printf("\n")
	}
}

func (m *Matrix) PrintStart() {
	fmt.Printf("%d-by-%d matrix:\n", m.rows, m.cols)
	for i := uint64(0); i < 2; i++ {
		for j := uint64(0); j < 2; j++ {
			fmt.Printf("%d ", m.data[i*m.cols+j])
		}
		fmt.Printf("\n")
	}
}

func (m *Matrix) Equals(n *Matrix) bool {
	if m.Cols() != n.Cols() {
		return false
	}
	if m.Rows() != n.Rows() {
		return false
	}
	return reflect.DeepEqual(m.data, n.data)
}

func (m Matrix) GobEncode() ([]byte, error) {
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

func (m *Matrix) GobDecode(buf []byte) error {
	b := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(b)
	err1 := decoder.Decode(&m.rows)
	err2 := decoder.Decode(&m.cols)

	m.data = make([]C.Elem, m.rows*m.cols)
	err3 := decoder.Decode(&m.data)

	if err1 != nil || err2 != nil || err3 != nil {
		panic("Gob decoding error")
	}

	return nil
}
