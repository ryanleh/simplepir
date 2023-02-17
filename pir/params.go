package pir

/*


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

		p := PickParamsGivenDimensions(l, m, n, logq)

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

func PickParamsGivenDimensions(l, m, n, logq uint64) *Params {
	p := &Params{
		N:    n,
                Logq: logq,
                L:    l,
                M:    m,
	}
        p.calcParams(false, m)
        return p
}*/
