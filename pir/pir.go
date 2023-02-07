package pir

import (
	"fmt"
	//"os"
	//"runtime"
	//"runtime/debug"
	//"runtime/pprof"
	//"time"
	// "math"
)

// Run full PIR scheme (offline + online phases).
func RunPIR(client *Client, server *Server, db *Database, p *Params, i uint64) {
	secret, query := client.Query(i)
	answer := server.Answer(query)

	val := client.Recover(secret, answer)

	if db.GetElem(i) != val {
		fmt.Printf("(querying index %d -- row should be >= %d): Got %d instead of %d\n",
			i, db.Data.Rows/4, val, db.GetElem(i))
		panic("Reconstruct failed!")
	}
}
