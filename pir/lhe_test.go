package pir

import (
	"testing"

	"github.com/henrycg/simplepir/matrix"
	"github.com/henrycg/simplepir/lwe"
	"github.com/henrycg/simplepir/rand"
)

func runLHE[T matrix.Elem](t *testing.T, client *Client[T], server *Server[T], db *Database[T], arr *matrix.Matrix[T]) {
	secret, query := client.QueryLHE(arr)
	answer := server.Answer(query)

	vals := client.RecoverManyLHE(secret, answer)
  shouldBe := matrix.Mul(db.Data, arr)
  shouldBe.ModConst(T(db.Info.P()))

  if !shouldBe.Equals(vals) {
    t.Fail()
  }
}

func testLHE[T matrix.Elem](t *testing.T, N uint64, d uint64) {
	prg := rand.NewRandomBufPRG()
	params := lwe.NewParamsFixedP(T(0).Bitlen(), N, 512)
	db := NewDatabaseRandomFixedParams[T](prg, N, d, params)
  arr := matrix.Rand[T](prg, db.Info.M, 1, params.P)

	server := NewServer(db)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runLHE(t, client, server, db, arr)
}

func testLHECompressed[T matrix.Elem](t *testing.T, N uint64, d uint64) {
	prg := rand.NewRandomBufPRG()
	params := lwe.NewParamsFixedP(T(0).Bitlen(), N, 512)
	db := NewDatabaseRandomFixedParams[T](prg, N, d, params)
  arr := matrix.Rand[T](prg, db.Info.M, 1, params.P)

	seed := rand.RandomPRGKey()
	server := NewServerSeed(db, seed)
	client := NewClient(server.Hint(), server.MatrixA(), db.Info)

	runLHE(t, client, server, db, arr)
}


func TestLHE32(t *testing.T) {
	testLHE[matrix.Elem32](t, uint64(1<<7)+3, uint64(9))
}

func TestLHE64(t *testing.T) {
	testLHE[matrix.Elem64](t, uint64(1<<8)+5, uint64(9))
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

func TestLHECompressed32(t *testing.T) {
	testLHECompressed[matrix.Elem32](t, uint64(1<<20), uint64(9))
}

func TestLHECompressed64(t *testing.T) {
	testLHECompressed[matrix.Elem64](t, uint64(1<<20), uint64(9))
}

func TestLHEBigDB32(t *testing.T) {
	testLHE[matrix.Elem32](t, uint64(1<<25), uint64(9))
}

func TestLHEBigDB64(t *testing.T) {
	testLHE[matrix.Elem64](t, uint64(1<<25), uint64(9))
}

func TestLHEBigDBCompressed32(t *testing.T) {
	testLHECompressed[matrix.Elem32](t, uint64(1<<25), uint64(9))
}

func TestLHEBigDBCompressed64(t *testing.T) {
	testLHECompressed[matrix.Elem64](t, uint64(1<<25), uint64(9))
}
