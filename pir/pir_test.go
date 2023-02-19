package pir

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/henrycg/simplepir/matrix"
	"github.com/henrycg/simplepir/lwe"
	"github.com/henrycg/simplepir/rand"
)

const SEC_PARAM = uint64(1 << 10)

func testServerEncode[T matrix.Elem](t *testing.T, N, d uint64) {
	prg := rand.NewRandomBufPRG()
	db := NewDatabaseRandom[T](prg, N, d)
	server := NewServer(db)

	var b bytes.Buffer
	enc := gob.NewEncoder(&b)
	err := enc.Encode(server)
	if err != nil {
		t.Fatal("Encoding failed")
	}

	dec := gob.NewDecoder(&b)
	var server2 Server[T]
	err = dec.Decode(&server2)
	if err != nil {
		t.Fatal("Decoding failed")
	}

	if *server2.params != *server.params {
		t.Fatal("Parameter mismatch")
	}
	if !server2.matrixA.Equals(server.matrixA) {
		t.Fatal("A matrix mismatch")
	}
	if !server2.hint.Equals(server.hint) {
		t.Fatal("Hint mismatch")
	}

	if server2.db.Info.Num != server.db.Info.Num {
		t.Fatal("DB info mismatch")
	}
	if server2.db.Info.Params.N != server.db.Info.Params.N {
		t.Fatal("DB info mismatch")
	}
	if !server2.db.Data.Equals(server.db.Data) {
		t.Fatal("DB mismatch")
	}
}

func TestServerEncode32(t *testing.T) {
	testServerEncode[matrix.Elem32](t, uint64(1<<20), uint64(8))
}

func TestServerEncode64(t *testing.T) {
	testServerEncode[matrix.Elem64](t, uint64(1<<20), uint64(8))
}

// Run full PIR scheme (offline + online phases).
func runPIR[T matrix.Elem](t *testing.T, client *Client[T], server *Server[T], db *Database[T], i uint64) {
	secret, query := client.Query(i)
	answer := server.Answer(query)

	val := client.Recover(secret, answer)

	if db.GetElem(i) != val {
		fmt.Printf("(querying index %d -- row should be >= %d): Got %d instead of %d\n",
			i, db.Data.Rows()/4, val, db.GetElem(i))
		t.Fatal()
	}
}

func runPIRmany[T matrix.Elem](t *testing.T, client *Client[T], server *Server[T], db *Database[T], i uint64) {
	secret, query := client.Query(i)
	answer := server.Answer(query)

	vals := client.RecoverMany(secret, answer)

	col_index := i % db.Info.M
	for row := uint64(0); row < uint64(len(vals)); row++ {
		index := row*db.Info.M + col_index
		if db.GetElem(index) != vals[row] {
			fmt.Printf("Querying index %d: Got %d instead of %d\n",
				index, vals[row], db.GetElem(index))
			t.Fatal("Reconstruct failed!")
		}
	}
}

func runLHE[T matrix.Elem](t *testing.T, client *Client[T], server *Server[T], db *Database[T], arr []uint64) {
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
			t.Fatal("Reconstruct failed!")
		}
	}
}

func testSimplePir[T matrix.Elem](t *testing.T, N uint64, d uint64, index uint64) {
	prg := rand.NewRandomBufPRG()
	db := NewDatabaseRandom[T](prg, N, d)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIR(t, client, server, db, index)
}

func testSimplePirMany[T matrix.Elem](t *testing.T, N uint64, d uint64, index uint64) {
	prg := rand.NewRandomBufPRG()
	db := NewDatabaseRandom[T](prg, N, d)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIRmany(t, client, server, db, index)
}

func testLHE[T matrix.Elem](t *testing.T, N uint64, d uint64) {
	prg := rand.NewRandomBufPRG()
	params := lwe.NewParamsFixedP(T(0).Bitlen(), N, 1024)
	db := NewDatabaseRandomFixedParams[T](prg, N, d, params)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	arr := RandArray(db.Info.M, db.Info.P())
	runLHE(t, client, server, db, arr)
}

func testSimplePirCompressed[T matrix.Elem](t *testing.T, N uint64, d uint64, index uint64) {
	prg := rand.NewRandomBufPRG()
	db := NewDatabaseRandom[T](prg, N, d)

	seed := rand.RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIR(t, client, server, db, index)
}

func testSimplePirCompressedMany[T matrix.Elem](t *testing.T, N uint64, d uint64, index uint64) {
	prg := rand.NewRandomBufPRG()
	db := NewDatabaseRandom[T](prg, N, d)

	seed := rand.RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runPIRmany(t, client, server, db, index)
}

func testLHECompressed[T matrix.Elem](t *testing.T, N uint64, d uint64) {
	prg := rand.NewRandomBufPRG()
	params := lwe.NewParamsFixedP(T(0).Bitlen(), N, 1024)
	db := NewDatabaseRandomFixedParams[T](prg, N, d, params)

	seed := rand.RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	arr := RandArray(db.Info.M, db.Info.P())
	runLHE(t, client, server, db, arr)
}

// Test SimplePIR correctness on DB with short entries.
func TestSimplePir32(t *testing.T) {
	testSimplePir[matrix.Elem32](t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePir64(t *testing.T) {
	testSimplePir[matrix.Elem64](t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirMany32(t *testing.T) {
	testSimplePirMany[matrix.Elem32](t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirMany64(t *testing.T) {
	testSimplePirMany[matrix.Elem64](t, uint64(1<<20), uint64(8), 262144)
}

func TestLHE32(t *testing.T) {
	testLHE[matrix.Elem32](t, uint64(1<<20), uint64(9))
}

func TestLHE64(t *testing.T) {
	testLHE[matrix.Elem64](t, uint64(1<<20), uint64(9))
}

func TestLHE32_2(t *testing.T) {
	testLHE[matrix.Elem32](t, uint64(1<<20), uint64(8))
}

func TestLHE64_2(t *testing.T) {
	testLHE[matrix.Elem64](t, uint64(1<<20), uint64(8))
}

func TestLHE32_3(t *testing.T) {
	testLHE[matrix.Elem32](t, uint64(1<<20), uint64(6))
}

func TestLHE64_3(t *testing.T) {
	testLHE[matrix.Elem64](t, uint64(1<<20), uint64(6))
}

func TestSimplePirCompressed32(t *testing.T) {
	testSimplePirCompressed[matrix.Elem32](t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirCompressed64(t *testing.T) {
	testSimplePirCompressed[matrix.Elem64](t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirCompressedMany32(t *testing.T) {
	testSimplePirCompressedMany[matrix.Elem32](t, uint64(1<<20), uint64(8), 262144)
}

func TestSimplePirCompressedMany64(t *testing.T) {
	testSimplePirCompressedMany[matrix.Elem64](t, uint64(1<<20), uint64(8), 262144)
}

func TestLHECompressed32(t *testing.T) {
	testLHECompressed[matrix.Elem32](t, uint64(1<<20), uint64(9))
}

func TestLHECompressed64(t *testing.T) {
	testLHECompressed[matrix.Elem64](t, uint64(1<<20), uint64(9))
}

// Test SimplePIR correctness on DB with long entries
func TestSimplePirLongRow32(t *testing.T) {
	testSimplePir[matrix.Elem32](t, uint64(1<<20), uint64(32), 1)
}

func TestSimplePirLongRow64(t *testing.T) {
	testSimplePir[matrix.Elem64](t, uint64(1<<20), uint64(64), 1)
}

func TestSimplePirLongRowCompressed32(t *testing.T) {
	testSimplePirCompressed[matrix.Elem32](t, uint64(1<<20), uint64(32), 1)
}

func TestSimplePirLongRowCompressed64(t *testing.T) {
	testSimplePirCompressed[matrix.Elem64](t, uint64(1<<20), uint64(64), 1)
}

// Test SimplePIR correctness on big DB
func TestSimplePirBigDB32(t *testing.T) {
	testSimplePir[matrix.Elem32](t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDB64(t *testing.T) {
	testSimplePir[matrix.Elem64](t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBmany32(t *testing.T) {
	testSimplePirMany[matrix.Elem32](t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBmany64(t *testing.T) {
	testSimplePirMany[matrix.Elem64](t, uint64(1<<25), uint64(7), 0)
}

func TestLHEBigDB32(t *testing.T) {
	testLHE[matrix.Elem32](t, uint64(1<<25), uint64(9))
}

func TestLHEBigDB64(t *testing.T) {
	testLHE[matrix.Elem64](t, uint64(1<<25), uint64(9))
}

func TestSimplePirBigDBCompressed32(t *testing.T) {
	testSimplePirCompressed[matrix.Elem32](t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBCompressed64(t *testing.T) {
	testSimplePirCompressed[matrix.Elem64](t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBCompressedMany32(t *testing.T) {
	testSimplePirCompressedMany[matrix.Elem32](t, uint64(1<<25), uint64(7), 0)
}

func TestSimplePirBigDBCompressedMany64(t *testing.T) {
	testSimplePirCompressedMany[matrix.Elem64](t, uint64(1<<25), uint64(7), 0)
}

func TestLHEBigDBCompressed32(t *testing.T) {
	testLHECompressed[matrix.Elem32](t, uint64(1<<25), uint64(9))
}

func TestLHEBigDBCompressed64(t *testing.T) {
	testLHECompressed[matrix.Elem64](t, uint64(1<<25), uint64(9))
}
