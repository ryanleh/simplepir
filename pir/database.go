package pir

import "math"
import "fmt"

import "github.com/henrycg/simplepir/lwe"
import "github.com/henrycg/simplepir/rand"
import "github.com/henrycg/simplepir/matrix"

type DBInfo struct {
	Num       uint64 // number of db entries.
	RowLength uint64 // number of bits per db entry.

	Packing uint64 // number of db entries per Z_p elem, if log(p) > db entry size.
	Ne      uint64 // number of Z_p elems per db entry, if db entry size > log(p).

	X uint64 // tunable param that governs communication,
	// must be in range [1, ne] and must be a divisor of ne;
	// represents the number of times the scheme is repeated.

	L uint64 // database width
	M uint64 // database height

	// For in-memory db compression
	Squishing uint64
	Cols      uint64

	Params *lwe.Params
}

type Database[T matrix.Elem] struct {
	Info *DBInfo
	Data *matrix.Matrix[T]
}

func (db *Database[T]) Copy() *Database[T] {
	return &Database[T]{
		Info: db.Info,
		Data: db.Data.Copy(),
	}
}

func (db *Database[T]) Squish() {
	//fmt.Printf("Original db dims: ")
	//db.Data.Dim()

	// Check that Params allow for this compression
	if !db.Data.CanSquish(db.Info.P()) {
		panic("Bad Params")
	}

	db.Info.Squishing = db.Data.SquishRatio()
	db.Info.Cols = db.Data.Cols()
	db.Data.Squish()
}

// Store the database with entries decomposed into Z_p elements, and mapped to [-p/2, p/2]
// Z_p elements that encode the same database entry are stacked vertically below each other.
func (Info *DBInfo) ReconstructElem(vals []uint64, index uint64) uint64 {
	q := uint64(1 << Info.Params.Logq)

	for i, _ := range vals {
		vals[i] = (vals[i] + Info.P()/2) % q
		vals[i] = vals[i] % Info.P()
	}

	val := Reconstruct_from_base_p(Info.P(), vals)

	if Info.Packing > 0 {
		val = Base_p((1 << Info.RowLength), val, index%Info.Packing)
	}

	return val
}

func (db *Database[T]) GetElem(i uint64) uint64 {
	if i >= db.Info.Num {
		panic("Index out of range")
	}

	cols := db.Data.Cols()
	col := i % cols
	row := i / cols

	if db.Info.Packing > 1 {
		new_i := i / db.Info.Packing
		col = new_i % cols
		row = new_i / cols
	}

	var vals []uint64
	for j := row * db.Info.Ne; j < (row+1)*db.Info.Ne; j++ {
		vals = append(vals, db.Data.Get(j, col))
	}

	return db.Info.ReconstructElem(vals, i)
}

// Returns how many Z_p elements are needed to represent a database of N entries,
// each consisting of row_length bits.
func numEntries(N, row_length, p uint64) (uint64, uint64, uint64) {
	if float64(row_length) <= math.Log2(float64(p)) {
		// pack multiple DB entries into a single Z_p elem
		logp := uint64(math.Log2(float64(p)))
		entries_per_elem := logp / row_length
		db_entries := uint64(math.Ceil(float64(N) / float64(entries_per_elem)))
		if db_entries == 0 || db_entries > N {
			fmt.Printf("Num entries is %d; N is %d\n", db_entries, N)
			panic("Should not happen")
		}
		return db_entries, 1, entries_per_elem
	}

	// use multiple Z_p elems to represent a single DB entry
	ne := Compute_num_entries_base_p(p, row_length)
	return N * ne, ne, 0
}

// Find smallest l, m such that l*m >= N*ne and ne divides l, where ne is
// the number of Z_p elements per db entry determined by row_length and p.
func approxSquareDatabaseDims(dbElems, elemsPerEntry, rowLength, p uint64) (uint64, uint64) {
	l := uint64(math.Floor(math.Sqrt(float64(dbElems))))

	rem := l % elemsPerEntry
	if rem != 0 {
		l += elemsPerEntry - rem
	}

	m := uint64(math.Ceil(float64(dbElems) / float64(l)))

	return l, m
}

func NewDBInfo(logq uint64, num uint64, rowLength uint64) *DBInfo {
	if (num == 0) || (rowLength == 0) {
		panic("Empty database!")
	}
	// Make a guess at plaintext modulus and compute parameters
	tempP := uint64(256)
	dbElems, elemsPerEntry, _ := numEntries(num, rowLength, tempP)
	_, m := approxSquareDatabaseDims(dbElems, elemsPerEntry, rowLength, tempP)

	params := lwe.NewParams(logq, m)
	if params == nil {
		panic("Could not find LWE Params")
	}

	return NewDBInfoFixedParams(num, rowLength, params, false)
}

func NewDBInfoFixedParams(num uint64, rowLength uint64, params *lwe.Params, fixed bool) *DBInfo {
	Info := &DBInfo{
		Num:       num,
		RowLength: rowLength,
		Params:    params,
	}

	// Compute database Info based on real LWE parameters
	var entriesPerElem uint64
	dbElems, elemsPerEntry, entriesPerElem := numEntries(num, rowLength, Info.Params.P)
	Info.L, Info.M = approxSquareDatabaseDims(dbElems, elemsPerEntry, rowLength, Info.Params.P)

	Info.Ne = elemsPerEntry
	Info.X = Info.Ne
	Info.Packing = entriesPerElem

	for Info.Ne%Info.X != 0 {
		Info.X += 1
	}

	fmt.Printf("Total packed db size is ~%f MB\n",
		float64(Info.L*Info.M)*math.Log2(float64(Info.P()))/(1024.0*1024.0*8.0))

	if dbElems > Info.L*Info.M {
		panic("lwe.Params and database size don't match")
	}

	if Info.L%Info.Ne != 0 {
		panic("Number of db elems per entry must divide db height")
	}

	if !fixed {
		// Recompute params based on chosen M
		Info.Params = lwe.NewParams(params.Logq, Info.M)
	}

	if Info.Params == nil {
		panic("Could not find good LWE Params")
	}

	return Info
}

// Number of Z_p elements to represent a DB record
func (Info *DBInfo) RecordSize() uint64 {
	return Info.Ne
}

func (Info *DBInfo) P() uint64 {
	return Info.Params.P
}

func NewDatabaseRandom[T matrix.Elem](prg *rand.BufPRGReader, logq, num, rowLength uint64) *Database[T] {
	info := NewDBInfo(logq, num, rowLength)
	return NewDatabaseRandomFixedParams[T](prg, num, rowLength, info.Params)
}

func NewDatabaseRandomFixedParams[T matrix.Elem](prg *rand.BufPRGReader, Num, rowLength uint64, params *lwe.Params) *Database[T] {
	db := new(Database[T])
	db.Info = NewDBInfoFixedParams(Num, rowLength, params, true)

	mod := db.Info.P()
	if ((1 << rowLength) < mod) && (db.Info.Packing == 1) {
		mod = (1 << rowLength)
	}

	db.Data = matrix.Rand[T](prg, db.Info.L, db.Info.M, 0, mod)

	// clear overflow cols
	row := db.Info.L - 1
	for i := Num; i < db.Info.L*db.Info.M; i++ {
		col := i % db.Info.M
		db.Data.Set(0, row, col)
	}

	// Map db elems to [-p/2; p/2]
	db.Data.SubUint64(db.Info.P() / 2)

	return db
}

func NewDatabase[T matrix.Elem](logq, num, rowLength uint64, vals []uint64) *Database[T] {
	info := NewDBInfo(logq, num, rowLength)
	return NewDatabaseFixedParams[T](num, rowLength, vals, info.Params)
}

func NewDatabaseFixedParams[T matrix.Elem](Num, rowLength uint64, vals []uint64, params *lwe.Params) *Database[T] {
	db := new(Database[T])
	db.Info = NewDBInfoFixedParams(Num, rowLength, params, true)
	db.Data = matrix.Zeros[T](db.Info.L, db.Info.M)

	if uint64(len(vals)) != Num {
		panic("Bad input db")
	}

	if db.Info.Packing > 0 {
		// Pack multiple db elems into each Z_p elem
		at := uint64(0)
		cur := uint64(0)
		coeff := uint64(1)
		for i, elem := range vals {
			cur += (elem * coeff)
			coeff *= (1 << rowLength)
			if ((i+1)%int(db.Info.Packing) == 0) || (i == len(vals)-1) {
				db.Data.Set(cur, at/db.Info.M, at%db.Info.M)
				at += 1
				cur = 0
				coeff = 1
			}
		}
	} else {
		// Use multiple Z_p elems to represent each db elem
		for i, elem := range vals {
			for j := uint64(0); j < db.Info.Ne; j++ {
				db.Data.Set(Base_p(db.Info.P(), elem, j), (uint64(i)/db.Info.M)*db.Info.Ne+j, uint64(i)%db.Info.M)
			}
		}
	}

	// Map db elems to [-p/2; p/2]
	db.Data.SubUint64(db.Info.P() / 2)

	return db
}
