package pir

import "math"
import "strings"
import "strconv"
import "fmt"
import _ "embed"

//go:embed params.csv
var lwe_params string

type Params struct {
	N     uint64  // LWE secret dimension
	Sigma float64 // LWE error distribution stddev

	L uint64 // DB height
	M uint64 // DB width

	Logq uint64 // (logarithm of) ciphertext modulus
	P    uint64 // plaintext modulus
}

func (p *Params) Delta() uint64 {
	return (1 << p.Logq) / (p.P)
}

func (p *Params) delta() uint64 {
	return uint64(math.Ceil(float64(p.Logq) / math.Log2(float64(p.P))))
}

func (p *Params) Round(x uint64) uint64 {
	Delta := p.Delta()
	v := (x + Delta/2) / Delta
	return v % p.P
}

func (p *Params) calcParams(doublepir bool, samples ...uint64) {
	if p.N == 0 || p.Logq == 0 {
		panic("Need to specify n and q!")
	}

	num_samples := uint64(0)
	for _, ns := range samples {
		if ns > num_samples {
			num_samples = ns
		}
	}

	lines := strings.Split(lwe_params, "\n")
	for _, l := range lines[1:] {
		line := strings.Split(l, ",")
		logn, _ := strconv.ParseUint(line[0], 10, 64)
		logm, _ := strconv.ParseUint(line[1], 10, 64)
		logq, _ := strconv.ParseUint(line[2], 10, 64)

		if (p.N == uint64(1<<logn)) &&
			(num_samples <= uint64(1<<logm)) &&
			(p.Logq == uint64(logq)) {
			sigma, _ := strconv.ParseFloat(line[3], 64)
			p.Sigma = sigma

			if doublepir {
				mod, _ := strconv.ParseUint(line[6], 10, 64)
				p.P = mod
			} else {
				mod, _ := strconv.ParseUint(line[5], 10, 64)
				p.P = mod
			}

			if sigma == 0.0 || p.P == 0 {
				panic("Params invalid!")
			}

			return
		}
	}

	fmt.Printf("Searched for %d, %d-by-%d, %d,\n", p.N, p.L, p.M, p.Logq)
	panic("No suitable params known!")
}

func (p *Params) PrintParams() {
	fmt.Printf("Working with: n=%d; db size=2^%d (l=%d, m=%d); logq=%d; p=%d; sigma=%f\n",
		p.N, int(math.Log2(float64(p.L))+math.Log2(float64(p.M))), p.L, p.M, p.Logq,
		p.P, p.Sigma)
}

func (p *Params) GetBW() {
	offline_download := float64(p.L*p.N*p.Logq) / (8.0 * 1024.0)
	fmt.Printf("\t\tOffline download: %d KB\n", uint64(offline_download))

	online_upload := float64(p.M*p.Logq) / (8.0 * 1024.0)
	fmt.Printf("\t\tOnline upload: %d KB\n", uint64(online_upload))

	online_download := float64(p.L*p.Logq) / (8.0 * 1024.0)
	fmt.Printf("\t\tOnline download: %d KB\n", uint64(online_download))
}

// Find smallest l, m such that l*m >= N*ne and ne divides l, where ne is
// the number of Z_p elements per db entry determined by row_length and p.
func approxSquareDatabaseDims(N, row_length, p uint64) (uint64, uint64) {
	db_elems, elems_per_entry, _ := Num_DB_entries(N, row_length, p)
	l := uint64(math.Floor(math.Sqrt(float64(db_elems))))

	rem := l % elems_per_entry
	if rem != 0 {
		l += elems_per_entry - rem
	}

	m := uint64(math.Ceil(float64(db_elems) / float64(l)))

	return l, m
}

func PickParams(N, d, n, logq uint64) *Params {
	var good_p *Params
	found := false

	// Iteratively refine p and DB dims, until find tight values
	for mod_p := uint64(2); ; mod_p += 1 {
		l, m := approxSquareDatabaseDims(N, d, mod_p)

		p := &Params{
			N:    n,
			Logq: logq,
			L:    l,
			M:    m,
		}
		p.calcParams(false, m)

		if p.P < mod_p {
			if !found {
				panic("Error; should not happen")
			}
			good_p.PrintParams()
			return good_p
		}

		good_p = p
		found = true
	}

	panic("Cannot be reached")
	return nil
}
