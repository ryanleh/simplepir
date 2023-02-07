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
func RunPIR(client *Client, server *Server, DB *Database, p Params, i uint64) {
  secret, query := client.Query(i)
	answer := server.Answer(query)

  val := client.Recover(secret, answer)

  if DB.GetElem(i) != val {
    fmt.Printf("(querying index %d -- row should be >= %d): Got %d instead of %d\n",
      i, DB.Data.Rows/4, val, DB.GetElem(i))
    panic("Reconstruct failed!")
  }
}

/*
// Run full PIR scheme (offline + online phases), where the transmission of the A matrix is compressed.
func RunPIRCompressed(pi *SimplePIR, DB *Database, p Params, i []uint64) (float64, float64) {
	fmt.Printf("Executing\n")
	//fmt.Printf("Memory limit: %d\n", debug.SetMemoryLimit(math.MaxInt64))
	debug.SetGCPercent(-1)

	num_queries := uint64(len(i))
	if DB.Data.Rows/num_queries < DB.Info.Ne {
		panic("Too many queries to handle!")
	}
	batch_sz := DB.Data.Rows / (DB.Info.Ne * num_queries) * DB.Data.Cols
	bw := float64(0)

	server_shared_state, comp_state := pi.InitCompressed(DB.Info, p)
	client_shared_state := pi.DecompressState(DB.Info, p, comp_state)

	fmt.Println("Setup...")
	start := time.Now()
	server_state, offline_download := pi.Setup(DB, server_shared_state, p)
	printTime(start)
	comm := float64(offline_download.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Printf("\t\tOffline download: %f KB\n", comm)
	bw += comm
	runtime.GC()

	fmt.Println("Building query...")
	start = time.Now()
	var client_state []State
	var query MsgSlice
	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		cs, q := pi.Query(index_to_query, client_shared_state, p, DB.Info)
		client_state = append(client_state, cs)
		query.Data = append(query.Data, q)
	}
	runtime.GC()
	printTime(start)
	comm = float64(query.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Printf("\t\tOnline upload: %f KB\n", comm)
	bw += comm
	runtime.GC()

	fmt.Println("Answering query...")
	start = time.Now()
	answer := pi.Answer(DB, query, server_state, server_shared_state, p)
	elapsed := printTime(start)
	rate := printRate(p, elapsed, len(i))
	comm = float64(answer.Size() * uint64(p.Logq) / (8.0 * 1024.0))
	fmt.Printf("\t\tOnline download: %f KB\n", comm)
	bw += comm
	runtime.GC()

	fmt.Println("Reconstructing...")
	start = time.Now()

	for index, _ := range i {
		index_to_query := i[index] + uint64(index)*batch_sz
		val := pi.Recover(index_to_query, uint64(index), offline_download,
			query.Data[index], answer, client_shared_state,
			client_state[index], p, DB.Info)

		if DB.GetElem(index_to_query) != val {
			fmt.Printf("Batch %d (querying index %d -- row should be >= %d): Got %d instead of %d\n",
				index, index_to_query, DB.Data.Rows/4, val, DB.GetElem(index_to_query))
			panic("Reconstruct failed!")
		}
	}
	fmt.Println("Success!")
	printTime(start)

	runtime.GC()
	debug.SetGCPercent(100)
	return rate, bw
}
*/
