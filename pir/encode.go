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

// Warning: does not store the A matrix (to save space)
func (s *Server[T]) GobEncode() ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(s.params)
	if err != nil {
		return buf.Bytes(), err
	}

	err = enc.Encode(s.db)
	if err != nil {
		return buf.Bytes(), err
	}

	err = enc.Encode(s.hint)
	return buf.Bytes(), err
}

func (s *Server[T]) GobDecode(buf []byte) error {
	b := bytes.NewBuffer(buf)
	dec := gob.NewDecoder(b)
	err := dec.Decode(&s.params)
	if err != nil {
		return err
	}

	err = dec.Decode(&s.db)
	if err != nil {
		return err
	}

	err = dec.Decode(&s.hint)
	return err
}
