package pir

import (
	"bytes"
	"encoding/gob"
)

func (q *Query[T]) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(*q.query)
	return buf.Bytes(), err
}

func (q *Query[T]) GobDecode(buf []byte) error {
	b := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(b)
	err := decoder.Decode(&q.query)
	return err
}

func (a *Answer[T]) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(*a.answer)
	return buf.Bytes(), err
}

func (a *Answer[T]) GobDecode(buf []byte) error {
	b := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(b)
	err := decoder.Decode(&a.answer)
	return err
}
