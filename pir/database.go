package pir

import "math"
import "fmt"

type DBInfo struct {
	Num        uint64 // number of db entries.
	Row_length uint64 // number of bits per db entry.

	Packing uint64 // number of db entries per Z_p elem, if log(p) > db entry size.
	Ne      uint64 // number of Z_p elems per db entry, if db entry size > log(p).

	X uint64 // tunable param that governs communication,
	// must be in range [1, ne] and must be a divisor of ne;
	// represents the number of times the scheme is repeated.
	P    uint64 // plaintext modulus.
	Logq uint64 // (logarithm of) ciphertext modulus.

	// For in-memory db compression
	Basis     uint64
	Squishing uint64
	Cols      uint64
}

type Database struct {
	Info *DBInfo
	Data *Matrix
}

func (db *Database) Copy() *Database {
	return &Database{
		Info: db.Info,
		Data: db.Data.Copy(),
	}
}

func (db *Database) Squish() {
	//fmt.Printf("Original db dims: ")
	//db.Data.Dim()

	db.Info.Basis = 10
	db.Info.Squishing = 3
	db.Info.Cols = db.Data.Cols
	db.Data.Squish(db.Info.Basis, db.Info.Squishing)

	//fmt.Printf("After squishing, with compression factor %d: ", db.Info.Squishing)
	//db.Data.Dim()

	// Check that params allow for this compression
	if (db.Info.P > (1 << db.Info.Basis)) || (db.Info.Logq < db.Info.Basis*db.Info.Squishing) {
		panic("Bad params")
	}
}

// Store the database with entries decomposed into Z_p elements, and mapped to [-p/2, p/2]
// Z_p elements that encode the same database entry are stacked vertically below each other.
func ReconstructElem(vals []uint64, index uint64, info *DBInfo) uint64 {
	q := uint64(1 << info.Logq)

	for i, _ := range vals {
		vals[i] = (vals[i] + info.P/2) % q
		vals[i] = vals[i] % info.P
	}

	val := Reconstruct_from_base_p(info.P, vals)

	if info.Packing > 0 {
		val = Base_p((1 << info.Row_length), val, index%info.Packing)
	}

	return val
}

func (db *Database) GetElem(i uint64) uint64 {
	if i >= db.Info.Num {
		panic("Index out of range")
	}

	col := i % db.Data.Cols
	row := i / db.Data.Cols

	if db.Info.Packing > 0 {
		new_i := i / db.Info.Packing
		col = new_i % db.Data.Cols
		row = new_i / db.Data.Cols
	}

	var vals []uint64
	for j := row * db.Info.Ne; j < (row+1)*db.Info.Ne; j++ {
		vals = append(vals, db.Data.Get(j, col))
	}

	return ReconstructElem(vals, i, db.Info)
}

// Find smallest l, m such that l*m >= N*ne and ne divides l, where ne is
// the number of Z_p elements per db entry determined by row_length and p.
func ApproxSquareDatabaseDims(N, row_length, p uint64) (uint64, uint64) {
	db_elems, elems_per_entry, _ := Num_DB_entries(N, row_length, p)
	l := uint64(math.Floor(math.Sqrt(float64(db_elems))))

	rem := l % elems_per_entry
	if rem != 0 {
		l += elems_per_entry - rem
	}

	m := uint64(math.Ceil(float64(db_elems) / float64(l)))

	return l, m
}

// Find smallest l, m such that l*m >= N*ne and ne divides l, where ne is
// the number of Z_p elements per db entry determined by row_length and p, and m >=
// lower_bound_m.
func ApproxDatabaseDims(N, row_length, p, lower_bound_m uint64) (uint64, uint64) {
	l, m := ApproxSquareDatabaseDims(N, row_length, p)
	if m >= lower_bound_m {
		return l, m
	}

	m = lower_bound_m
	db_elems, elems_per_entry, _ := Num_DB_entries(N, row_length, p)
	l = uint64(math.Ceil(float64(db_elems) / float64(m)))

	rem := l % elems_per_entry
	if rem != 0 {
		l += elems_per_entry - rem
	}

	return l, m
}

func NewDBInfo(num, row_length uint64, p *Params) *DBInfo {
	if (num == 0) || (row_length == 0) {
		panic("Empty database!")
	}

	info := new(DBInfo)

	info.Num = num
	info.Row_length = row_length
	info.P = p.P
	info.Logq = p.Logq

	db_elems, elems_per_entry, entries_per_elem := Num_DB_entries(num, row_length, p.P)
	info.Ne = elems_per_entry
	info.X = info.Ne
	info.Packing = entries_per_elem

	for info.Ne%info.X != 0 {
		info.X += 1
	}

	info.Basis = 0
	info.Squishing = 0

	fmt.Printf("Total packed db size is ~%f MB\n",
		float64(p.L*p.M)*math.Log2(float64(p.P))/(1024.0*1024.0*8.0))

	if db_elems > p.L*p.M {
		panic("Params and database size don't match")
	}

	if p.L%info.Ne != 0 {
		panic("Number of db elems per entry must divide db height")
	}

	return info
}

func NewDatabaseRandom(prg *BufPRGReader, Num, row_length uint64, p *Params) *Database {
	db := new(Database)
	db.Info = NewDBInfo(Num, row_length, p)
	db.Data = MatrixRand(prg, p.L, p.M, 0, p.P)

	// Map db elems to [-p/2; p/2]
	db.Data.Sub(p.P / 2)

	return db
}

func NewDatabase(Num, row_length uint64, p *Params, vals []uint64) *Database {
	db := new(Database)
	db.Info = NewDBInfo(Num, row_length, p)
	db.Data = MatrixZeros(p.L, p.M)

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
			coeff *= (1 << row_length)
			if ((i+1)%int(db.Info.Packing) == 0) || (i == len(vals)-1) {
				db.Data.Set(cur, at/p.M, at%p.M)
				at += 1
				cur = 0
				coeff = 1
			}
		}
	} else {
		// Use multiple Z_p elems to represent each db elem
		for i, elem := range vals {
			for j := uint64(0); j < db.Info.Ne; j++ {
				db.Data.Set(Base_p(db.Info.P, elem, j), (uint64(i)/p.M)*db.Info.Ne+j, uint64(i)%p.M)
			}
		}
	}

	// Map db elems to [-p/2; p/2]
	db.Data.Sub(p.P / 2)

	return db
}
