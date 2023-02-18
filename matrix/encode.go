package matrix

import (
	"bytes"
  "fmt"
	"encoding/gob"
)

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
