package pir

import "github.com/henrycg/simplepir/matrix"

//import "fmt"

type Server struct {
	params  *Params
	matrixA *matrix.Matrix

	db   *Database
	hint *matrix.Matrix
}

type Client struct {
	prg *BufPRGReader

	params *Params
	hint   *matrix.Matrix

	matrixA *matrix.Matrix
	dbinfo  *DBInfo
}

type Query = matrix.Matrix
type Secret struct {
	query  *Query
	secret *matrix.Matrix
	index  uint64
}

type Answer = matrix.Matrix

func NewServer(params *Params, db *Database) *Server {
	prg := NewRandomBufPRG()
	matrixA := matrix.MatrixRand(prg, params.M, params.N, params.Logq, 0)
	return setupServer(params, db, matrixA)
}

func NewServerSeed(params *Params, db *Database, seed *PRGKey) *Server {
	prg := NewBufPRG(NewPRG(seed))
	matrixA := matrix.MatrixRand(prg, params.M, params.N, params.Logq, 0)
	return setupServer(params, db, matrixA)
}

func setupServer(params *Params, db *Database, matrixA *matrix.Matrix) *Server {
	s := &Server{
		params:  params,
		matrixA: matrixA,
		db:      db.Copy(),
		hint:    matrix.MatrixMul(db.Data, matrixA),
	}

	// map the database entries to [0, p] (rather than [-p/1, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	s.db.Data.Add(s.params.P / 2)
	s.db.Squish()

	return s
}

func (s *Server) Hint() *matrix.Matrix {
	return s.hint
}

func (s *Server) MatrixA() *matrix.Matrix {
	return s.matrixA
}

func NewClient(params *Params, hint *matrix.Matrix, matrixA *matrix.Matrix, dbinfo *DBInfo) *Client {
	return &Client{
		prg: NewRandomBufPRG(),

		params: params,
		hint:   hint.Copy(),

		matrixA: matrixA.Copy(),
		dbinfo:  dbinfo,
	}
}

func (c *Client) Query(i uint64) (*Secret, *Query) {
	s := &Secret{
		secret: matrix.MatrixRand(c.prg, c.params.N, 1, c.params.Logq, 0),
		index:  i,
	}

	err := matrix.MatrixGaussian(c.prg, c.params.M, 1)

	query := matrix.MatrixMul(c.matrixA, s.secret)
	query.MatrixAdd(err)
	query.Data[i%c.params.M] += matrix.Elem(c.params.Delta())

	// Pad the query to match the dimensions of the compressed DB
	if c.params.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.params.M % c.dbinfo.Squishing))
	}

	s.query = query.Copy()

	return s, query
}

func (s *Server) Answer(query *Query) *Answer {
	return matrix.MatrixMulVecPacked(s.db.Data,
		query,
		s.db.Info.Basis,
		s.db.Info.Squishing)
}

func (c *Client) Recover(secret *Secret, ans *Answer) uint64 {
	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.params.M; j++ {
		offset += ratio * secret.query.Get(j, 0)
	}
	offset %= (1 << c.params.Logq)
	offset = (1 << c.params.Logq) - offset

	row := secret.index / c.params.M
	interm := matrix.MatrixMul(c.hint, secret.secret)
	ans.MatrixSub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := uint64(ans.Data[j]) + offset
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.MatrixAdd(interm)

	return ReconstructElem(vals, secret.index, c.dbinfo)
}
