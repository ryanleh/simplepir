package matrix

import (
	"fmt"
	"encoding/gob"
	"bytes"
	"testing"
)

func TestGob(t *testing.T) {
	m := New(5, 5)
	m.Set(1, 0, 0)
	m.Set(2, 0, 1)
	m.Set(3, 0, 2)

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(m)
	if err != nil {
		fmt.Println(err)
		panic("Encoding failed")
	}

	dec := gob.NewDecoder(&buf)
	var n Matrix
	err = dec.Decode(&n)
	if err != nil {
		fmt.Println(err)
		panic("Decoding failed")
	}

	if ! m.Equals(&n) {
		m.Print()
		n.Print()
		panic("Objects are not equal")
	}
}
