package lwe

import "math"
import "fmt"

var secretDimension = uint64(1024)
var lweErrorStdDev = float64(6.4)
var logCiphertextModulus = uint64(32)

/* Maps #samples ==> plaintext modulus */
var plaintextModulus = map[uint64]uint64{
  1<<13: 991,
  1<<14: 833,
  1<<15: 701,
  1<<16: 589,
  1<<17: 495,
  1<<18: 416,
  1<<19: 350,
  1<<20: 294,
  1<<21: 247,
}


type Params struct {
	N     uint64  // LWE secret dimension
	Sigma float64 // LWE error distribution stddev
	M     uint64  // LWE samples supported

	Logq uint64 // (logarithm of) ciphertext modulus
	P    uint64 // plaintext modulus
}

func (p *Params) Delta() uint64 {
	return (1 << p.Logq) / (p.P)
}

func (p *Params) Round(x uint64) uint64 {
	Delta := p.Delta()
	v := (x + Delta/2) / Delta
	return v % p.P
}

func (p *Params) PrintParams() {
	fmt.Printf("Working with: n=%d; m=%d; logq=%d; p=%d; sigma=%f\n",
		p.N, p.M, p.Logq, p.P, p.Sigma)
}

// Output LWE parameters for Regev encryption where
// each ciphertext can support up to 'nSamples' 
// homomorphic additions. 
func NewParams(nSamples uint64) *Params {
  max := uint64(math.MaxUint64)
  m := max
  pmod := uint64(0)
  for mNew,pNew := range plaintextModulus {
    if mNew < m && nSamples < mNew {
      m = mNew
      pmod = pNew
    }
  }

  // No good parameters found
  if m == max {
    return nil
  }

  return &Params{
    N: secretDimension, 
    Sigma: lweErrorStdDev,
    Logq: 32,
    M: m,
    P: pmod,
  }
}

func NewParamsFixedP(nSamples uint64, pMod uint64) *Params {
  return &Params{
    N: secretDimension, 
    Sigma: lweErrorStdDev,
    Logq: 32,
    M: nSamples,
    P: pMod,
  }
}

