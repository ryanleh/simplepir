package matrix

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

  "github.com/henrycg/simplepir/rand"
)

func TestGob(t *testing.T) {
	m := New[Elem32](5, 5)
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
	var n Matrix[Elem32]
	err = dec.Decode(&n)
	if err != nil {
		fmt.Println(err)
		panic("Decoding failed")
	}

	if !m.Equals(&n) {
		m.Print()
		n.Print()
		panic("Objects are not equal")
	}
}

func testAdd[U Elem](t *testing.T, logq uint64, r1 uint64, c1 uint64) {
  rand := rand.NewRandomBufPRG()

  m := Rand[U](rand, r1, c1, 0)
  z := Zeros[U](r1, c1)

  if !z.Equals(z) {
    t.Fail()
  }

  z.Add(m)
  if !z.Equals(m) {
    t.Fail()
  }
}

func TestAdd32(t *testing.T) {
  testAdd[Elem32](t, 32, 2, 2)
}

func TestAdd64(t *testing.T) {
  testAdd[Elem64](t, 32, 72, 110)
}

func testMul[U Elem](t *testing.T, logq uint64, r1 uint64, c1 uint64, r2 uint64, c2 uint64) {
  rand := rand.NewRandomBufPRG()

  m1 := Rand[U](rand, r1, c1, 12309)
  m2 := Rand[U](rand, r2, c2, 9123)
  z := Zeros[U](r2, c2)
  zout := Zeros[U](r1, c2)

  z2 := Mul(m1, z)
  if !z2.Equals(zout) {
    t.Fail()
  }

  out := Mul(m1, m2)
  res := Zeros[U](r1, c2)
  for i := uint64(0); i < r1; i++ {
    for j := uint64(0); j < c2; j++ {
      tmp := U(0)
      for k := uint64(0); k < c1; k++ {
        tmp += (U(m1.Get(i, k)) * U(m2.Get(k, j)))
      }
      res.Set(i, j, tmp)
    }
  }

  if !out.Equals(res) {
    t.Fail()
  }
}

func TestMul32(t *testing.T) {
  testMul[Elem32](t, 32, 2, 8, 8, 7)
}

func TestMul64(t *testing.T) {
  testMul[Elem64](t, 32, 2, 8, 8, 7)
}

func testGauss[U Elem](t *testing.T, r1 uint64, c1 uint64) {
  rand := rand.NewRandomBufPRG()
  Gaussian[U](rand, r1, c1)
}

func TestGauss32(t *testing.T) {
  testGauss[Elem32](t, 2, 8)
}

func TestGauss64(t *testing.T) {
  testGauss[Elem64](t, 2, 8)
}
