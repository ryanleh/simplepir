package lwe

//import "fmt"
//import "math/rand"
import "testing"

func TestGood64(t *testing.T) {
	p := NewParams(64, 100)
	if p == nil {
		t.Fail()
	}

	if p.P != uint64(95640378) || p.Logq != 64 || p.M !=
		8192000 || p.Sigma != float64(5.0) || p.N !=
		uint64(4096) {
		t.Fail()
	}
}

/*
func TestGauss64(t *testing.T) {
  r := rand.New(rand.NewSource(99))
  bins := make([]int, 1000)

  for i := 0; i<1024*1024*4; i++ {
    v := GaussSample64(r)
    v = (v / 8000) + 500
    bins[v] += 1
  }

  for i := 0; i<len(bins); i++ {
    fmt.Printf("bin[%v] = %v\n", i-500, bins[i])
  }
}
*/
