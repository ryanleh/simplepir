package pir

import (
	//"log"
	"testing"
	"github.com/henrycg/simplepir/matrix"
)

func TestGauss(t *testing.T) {
	prg := NewBufPRG(NewPRG(RandomPRGKey()))

	buckets := make([]int, 256)
	for i := 0; i < 1000000; i++ {
		buckets[matrix.GaussSample(prg)+128] += 1
	}

	/*
		for i := 0; i < len(buckets); i++ {
			log.Printf("bucket[%v] = %v", i, buckets[i])
		}
	*/
}
