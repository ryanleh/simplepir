package pir

import (
	//"encoding/csv"
	"fmt"
	//"math"
	//"os"
	//"strconv"
	"testing"
)

const LOGQ = uint64(32)
const SEC_PARAM = uint64(1 << 10)

// Run full PIR scheme (offline + online phases).
func runPIR(client *Client, server *Server, db *Database, p *Params, i uint64) {
	secret, query := client.Query(i)
	answer := server.Answer(query)

	val := client.Recover(secret, answer)

	if db.GetElem(i) != val {
		fmt.Printf("(querying index %d -- row should be >= %d): Got %d instead of %d\n",
			i, db.Data.Rows()/4, val, db.GetElem(i))
		panic("Reconstruct failed!")
	}
}

func testDBInit(t *testing.T, N uint64, d uint64, vals []uint64) *Database {
	p := PickParams(N, d, SEC_PARAM, LOGQ)
	db := NewDatabase(N, d, p, vals)

	for i := uint64(0); i < N; i++ {
		if db.GetElem(i) != (i + 1) {
			t.FailNow()
		}
	}

	return db
}

// Test that DB packing methods are correct, when each database entry is ~ 1 Z_p elem.
func TestDBMediumEntries(t *testing.T) {
	vals := []uint64{1, 2, 3, 4}
	db := testDBInit(t, uint64(4), uint64(9), vals)

	if db.Info.Packing != 1 || db.Info.Ne != 1 {
		t.FailNow()
	}
}

// Test that DB packing methods are correct, when multiple database entries fit in 1 Z_p elem.
func TestDBSmallEntries(t *testing.T) {
	vals := []uint64{1, 2, 3, 4}
	db := testDBInit(t, uint64(4), uint64(3), vals)

	if db.Info.Packing <= 1 || db.Info.Ne != 1 {
		t.FailNow()
	}
}

// Test that DB packing methods are correct, when each database entry requires multiple Z_p elems.
func TestDBLargeEntries(t *testing.T) {
	vals := []uint64{1, 2, 3, 4}
	db := testDBInit(t, uint64(4), uint64(12), vals)

	if db.Info.Packing != 0 || db.Info.Ne <= 1 {
		panic("Should not happen.")
	}
}

func testSimplePir(t *testing.T, N uint64, d uint64, index uint64) {
	prg := NewRandomBufPRG()
	p := PickParams(N, d, SEC_PARAM, LOGQ)
	db := NewDatabaseRandom(prg, N, d, p)

	server := NewServer(p, db)
	client := NewClient(p, server.Hint(), server.MatrixA(), db.Info)

	runPIR(client, server, db, p, index)
}

func testSimplePirCompressed(t *testing.T, N uint64, d uint64, index uint64) {
	prg := NewRandomBufPRG()
	p := PickParams(N, d, SEC_PARAM, LOGQ)
	db := NewDatabaseRandom(prg, N, d, p)

	seed := RandomPRGKey()
	server := NewServerSeed(p, db, seed)
	client := NewClient(p, server.Hint(), server.MatrixA(), db.Info)

	runPIR(client, server, db, p, index)
}

// Test SimplePIR correctness on DB with short entries.
func TestSimplePir(t *testing.T) {
	testSimplePir(t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirCompressed(t *testing.T) {
	testSimplePirCompressed(t, uint64(1<<20), uint64(8), 262144)
}

// Test SimplePIR correctness on DB with long entries
func TestSimplePirLongRow(t *testing.T) {
	testSimplePir(t, uint64(1<<20), uint64(32), 1)
}

func TestSimplePirLongRowCompressed(t *testing.T) {
	testSimplePirCompressed(t, uint64(1<<20), uint64(32), 1)
}

// Test SimplePIR correctness on big DB
func TestSimplePirBigDB(t *testing.T) {
	testSimplePir(t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBCompressed(t *testing.T) {
	testSimplePirCompressed(t, uint64(1<<25), uint64(7), 0)
}
