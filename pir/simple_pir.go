package pir

// #cgo CFLAGS: -O3 -march=native
// #include "pir.h"
import "C"
import "fmt"

type Server struct {
  params Params

  matrixA *Matrix
  prg *BufPRGReader

  db *Database
  hint *Matrix
}

type Client struct {
  prg *BufPRGReader

  params Params
  hint *Matrix

  matrixA *Matrix
  dbinfo *DBInfo
}

type Query = Matrix
type Secret struct {
  secret *Matrix
  index uint64
}

type Answer = Matrix

func pickParams(N, d, n, logq uint64) Params {
	good_p := Params{}
	found := false

	// Iteratively refine p and DB dims, until find tight values
	for mod_p := uint64(2); ; mod_p += 1 {
		l, m := ApproxSquareDatabaseDims(N, d, mod_p)

		p := Params{
			N:    n,
			Logq: logq,
			L:    l,
			M:    m,
		}
		p.PickParams(false, m)

		if p.P < mod_p {
			if !found {
				panic("Error; should not happen")
			}
			good_p.PrintParams()
			return good_p
		}

		good_p = p
		found = true
	}

	panic("Cannot be reached")
	return Params{}
}


func NewServer(N, d, n, logq uint64) *Server {
  s := new(Server)
  s.params = pickParams(N, d, n, logq)
  prg := NewBufPRG(NewPRG(RandomPRGKey()))
	s.matrixA = MatrixRand(prg, p.M, p.N, p.Logq, 0)
  return s
}

func NewServerSeed(N, d, n, logq uint64, seed *PRGKey) *Server {
  s := new(Server)
  s.params = pickParams(N, d, n, logq)
	s.prg = NewBufPRG(NewPRG(seed))
	s.matrixA = MatrixRand(s.prg, p.M, p.N, p.Logq, 0)
  return s
}

func (s *Server) SetDatabase(DB *Database) {
  s.db = DB.Copy()
	s.hint = MatrixMul(s.db.Data, s.matrixA)

	// map the database entries to [0, p] (rather than [-p/1, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	s.db.Data.Add(s.params.P / 2)
	s.db.Squish()
}

func NewClient(params Params, hint *Matrix, matrixA *Matrix, dbinfo *DBInfo) *Client {
  return &Client {
    params: params, 
    hint: hint.Copy(),  

    matrixA: matrixA.Copy(), 
    dbinfo: dbinfo,
  }
}

func (c *Client) Query(i uint64) (*Secret, *Query) {
  s := &Secret{
    secret: MatrixRand(c.params.N, 1, c.params.Logq, 0),
    index: i,
  }

	err := MatrixGaussian(c.params.M, 1)

	query := MatrixMul(c.matrixA, s.secret)
	query.MatrixAdd(err)
	query.Data[i % c.params.M] += C.Elem(c.params.Delta())

	// Pad the query to match the dimensions of the compressed DB
	if c.params.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.params.M % c.dbinfo.Squishing))
	}

	return secret, query
}

func (s *Server) Answer(query Query) *Answer {
	ans := new(Matrix)
	batch_sz := s.DB.Data.Rows / num_queries // how many rows of the database each query in the batch maps to

  return MatrixMulVecPacked(s.db.Data,
    q.Data,
    s.db.Info.Basis,
    s.db.Info.Squishing)
}

func (c *Client) Recover(secret *Secret, ans *Answer) uint64 {
	ratio := p.P / 2
	offset := uint64(0)
	for j := uint64(0); j < p.M; j++ {
		offset += ratio * query.Data[0].Get(j, 0)
	}
	offset %= (1 << p.Logq)
	offset = (1 << p.Logq) - offset

	row := secret.index / p.M
	interm := MatrixMul(c.hint, secret.secret)
	ans.MatrixSub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * info.Ne; j < (row+1)*info.Ne; j++ {
		noised := uint64(ans.Data[j]) + offset
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.MatrixAdd(interm)

	return ReconstructElem(vals, secret.i, c.dbinfo)
}

