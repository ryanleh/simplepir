package lwe

import "testing"

func TestTooManySamples(t *testing.T) {
  p := NewParams(1000000000)
  if p != nil {
    t.Fail()
  }
}

func TestGood(t *testing.T) {
  p := NewParams(10)
  if p == nil {
    t.Fail()
  }

  if p.P != uint64(991) || p.Logq != 32 || p.M !=
  1 << 13 || p.Sigma != float64(6.4) || p.N !=
  uint64(1024) {
    t.Fail()
  }
}

func TestDelta(t *testing.T) {
  p := NewParams(10)
  if p.Delta() != 4333973 {
    t.Fail()
  }
}
