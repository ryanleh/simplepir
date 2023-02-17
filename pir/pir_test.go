package pir

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/henrycg/simplepir/lwe"
)

const LOGQ = uint64(32)
const SEC_PARAM = uint64(1 << 10)

func testServerEncode(t *testing.T, N, d uint64) {
	prg := NewRandomBufPRG()
	db := NewDatabaseRandom(prg, N, d)
	server := NewServer(db)

	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(server)
	if err != nil {
		panic("Encoding failed")
	}

	dec := gob.NewDecoder(&b)
	var server2 Server
	err = dec.Decode(&server2)
	if err != nil {
		panic("Decoding failed")
	}

	if *server2.params != *server.params {
		panic("Parameter mismatch")
	}
	if !server2.matrixA.Equals(server.matrixA) {
		panic("A matrix mismatch")
	}
	if !server2.hint.Equals(server.hint) {
		panic("Hint mismatch")
	}

	if server2.db.Info.Num != server.db.Info.Num {
		panic("DB info mismatch")
	}
	if server2.db.Info.Params.N != server.db.Info.Params.N {
		panic("DB info mismatch")
	}
	if !server2.db.Data.Equals(server.db.Data) {
		panic("DB mismatch")
	}
}

func TestServerEncode(t *testing.T) {
	testServerEncode(t, uint64(1<<20), uint64(8))
}

// Run full PIR scheme (offline + online phases).
func runPIR(client *Client, server *Server, db *Database, i uint64) {
	secret, query := client.Query(i)
	answer := server.Answer(query)

	val := client.Recover(secret, answer)

	if db.GetElem(i) != val {
		fmt.Printf("(querying index %d -- row should be >= %d): Got %d instead of %d\n",
			i, db.Data.Rows()/4, val, db.GetElem(i))
		panic("Reconstruct failed!")
	}
}

func runPIRmany(client *Client, server *Server, db *Database, i uint64) {
	secret, query := client.Query(i)
	answer := server.Answer(query)

	vals := client.RecoverMany(secret, answer)

	col_index := i % db.Info.M
	for row := uint64(0); row < uint64(len(vals)); row++ {
		index := row*db.Info.M + col_index
		if db.GetElem(index) != vals[row] {
			fmt.Printf("Querying index %d: Got %d instead of %d\n",
				index, vals[row], db.GetElem(index))
			panic("Reconstruct failed!")
		}
	}
}

func runLHE(client *Client, server *Server, db *Database, arr []uint64) {
	secret, query := client.QueryLHE(arr)
	answer := server.Answer(query)

	vals := client.RecoverManyLHE(secret, answer)

	at := uint64(0)
	mod := db.Info.P()
	for i := 0; i < len(vals); i++ {
		should_be := uint64(0)
		for j := uint64(0); (j < uint64(len(arr))) && (at < db.Info.Num); j++ {
			should_be += arr[j] * db.GetElem(at)
			at += 1
		}
		should_be %= mod

		if should_be != vals[i] {
			fmt.Printf("Row %d: Got %d instead of %d (mod %d)\n",
				i, vals[i], should_be, mod)
			panic("Reconstruct failed!")
		}
	}
}

func testDBInit(t *testing.T, N uint64, d uint64, vals []uint64) *Database {
	db := NewDatabase(N, d, vals)

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
	db := NewDatabaseRandom(prg, N, d)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIR(client, server, db, index)
}

func testSimplePirMany(t *testing.T, N uint64, d uint64, index uint64) {
	prg := NewRandomBufPRG()
	db := NewDatabaseRandom(prg, N, d)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIRmany(client, server, db, index)
}

func testLHE(t *testing.T, N uint64, d uint64) {
	prg := NewRandomBufPRG()
	params := lwe.NewParamsFixedP(N, 1024)
	db := NewDatabaseRandomFixedParams(prg, N, d, params)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	arr := RandArray(db.Info.M, db.Info.P())
	runLHE(client, server, db, arr)
}

func testSimplePirCompressed(t *testing.T, N uint64, d uint64, index uint64) {
	prg := NewRandomBufPRG()
	db := NewDatabaseRandom(prg, N, d)

	seed := RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIR(client, server, db, index)
}

func testSimplePirCompressedMany(t *testing.T, N uint64, d uint64, index uint64) {
	prg := NewRandomBufPRG()
	db := NewDatabaseRandom(prg, N, d)

	seed := RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIRmany(client, server, db, index)
}

func testLHECompressed(t *testing.T, N uint64, d uint64) {
	prg := NewRandomBufPRG()
	params := lwe.NewParamsFixedP(N, 1024)
	db := NewDatabaseRandomFixedParams(prg, N, d, params)

	seed := RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	arr := RandArray(db.Info.M, db.Info.P())
	runLHE(client, server, db, arr)
}

// Test SimplePIR correctness on DB with short entries.
func TestSimplePir(t *testing.T) {
	testSimplePir(t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirMany(t *testing.T) {
	testSimplePirMany(t, uint64(1<<20), uint64(8), 262144)
}

func TestLHE(t *testing.T) {
	testLHE(t, uint64(1<<20), uint64(9))
}

func TestLHE2(t *testing.T) {
	testLHE(t, uint64(1<<20), uint64(8))
}

func TestLHE3(t *testing.T) {
	testLHE(t, uint64(1<<20), uint64(6))
}

func TestSimplePirCompressed(t *testing.T) {
	testSimplePirCompressed(t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirCompressedMany(t *testing.T) {
	testSimplePirCompressedMany(t, uint64(1<<20), uint64(8), 262144)
}

func TestLHECompressed(t *testing.T) {
	testLHECompressed(t, uint64(1<<20), uint64(9))
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

func TestSimplePirBigDBmany(t *testing.T) {
	testSimplePirMany(t, uint64(1<<25), uint64(7), 0)
}

func TestLHEBigDB(t *testing.T) {
	testLHE(t, uint64(1<<25), uint64(9))
}

func TestSimplePirBigDBCompressed(t *testing.T) {
	testSimplePirCompressed(t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBCompressedMany(t *testing.T) {
	testSimplePirCompressedMany(t, uint64(1<<25), uint64(7), 0)
}

func TestLHEBigDBCompressed(t *testing.T) {
	testLHECompressed(t, uint64(1<<25), uint64(9))
}
