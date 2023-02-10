package pir

import "github.com/henrycg/simplepir/matrix"


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
type SecretLHE struct {
	query  *Query
	secret *matrix.Matrix
	arr    []uint64
}

type Answer = matrix.Matrix

func NewServer(params *Params, db *Database) *Server {
	prg := NewRandomBufPRG()
	matrixA := matrix.Rand(prg, params.M, params.N, params.Logq, 0)
	return setupServer(params, db, matrixA)
}

func NewServerSeed(params *Params, db *Database, seed *PRGKey) *Server {
	prg := NewBufPRG(NewPRG(seed))
	matrixA := matrix.Rand(prg, params.M, params.N, params.Logq, 0)
	return setupServer(params, db, matrixA)
}

func setupServer(params *Params, db *Database, matrixA *matrix.Matrix) *Server {
	s := &Server{
		params:  params,
		matrixA: matrixA,
		db:      db.Copy(),
		hint:    matrix.Mul(db.Data, matrixA),
	}

	// map the database entries to [0, p] (rather than [-p/1, p/2]) and then
	// pack the database more tightly in memory, because the online computation
	// is memory-bandwidth-bound
	s.db.Data.AddUint64(s.params.P / 2)
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
		secret: matrix.Rand(c.prg, c.params.N, 1, c.params.Logq, 0),
		index:  i,
	}

	err := matrix.Gaussian(c.prg, c.params.M, 1)

	query := matrix.Mul(c.matrixA, s.secret)
	query.Add(err)
	query.AddAt(c.params.Delta(), i%c.params.M, 0)

	// Pad the query to match the dimensions of the compressed DB
	if c.params.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.params.M % c.dbinfo.Squishing))
	}

	s.query = query.Copy()

	return s, query
}

func (c *Client) QueryLHE(arr []uint64) (*SecretLHE, *Query) {
	if uint64(len(arr)) != c.params.M {
		panic("Parameter mismatch")
	}

	if (c.dbinfo.Packing != 1) || (c.dbinfo.Ne != 1) || ((1 << c.dbinfo.Row_length) != c.dbinfo.P) {
		panic("Not yet supported.")
	}

	// checks that p is a power of 2 (since q must be)
	if (c.params.P & (c.params.P-1)) != 0 {
		panic("LHE requires p | q.")
	}

	s := &SecretLHE{
		secret: matrix.Rand(c.prg, c.params.N, 1, c.params.Logq, 0),
		arr:  arr,
	}

	err := matrix.Gaussian(c.prg, c.params.M, 1)

	query := matrix.Mul(c.matrixA, s.secret)
	query.Add(err)
	for j:=uint64(0); j<c.params.M; j++ {
		query.AddAt(c.params.Delta() * arr[j], j, 0)
	}

	// Pad the query to match the dimensions of the compressed DB
	if c.params.M%c.dbinfo.Squishing != 0 {
		query.AppendZeros(c.dbinfo.Squishing - (c.params.M % c.dbinfo.Squishing))
	}

	s.query = query.Copy()

	return s, query
}

func (s *Server) Answer(query *Query) *Answer {
	return matrix.MulVecPacked(s.db.Data,
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
	interm := matrix.Mul(c.hint, secret.secret)
	ans.Sub(interm)

	var vals []uint64
	// Recover each Z_p element that makes up the desired database entry
	for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
		noised := ans.Get(j, 0) + offset
		denoised := c.params.Round(noised)
		vals = append(vals, denoised)
		//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
	}
	ans.Add(interm)

	return ReconstructElem(vals, secret.index, c.dbinfo)
}

func (c *Client) RecoverMany(secret *Secret, ans *Answer) []uint64 {
	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.params.M; j++ {
		offset += ratio * secret.query.Get(j, 0)
	}
	offset %= (1 << c.params.Logq)
	offset = (1 << c.params.Logq) - offset

	interm := matrix.Mul(c.hint, secret.secret)
	ans.Sub(interm)

	num_rows := ans.Rows() / c.dbinfo.Ne
	i := secret.index % c.params.M
	out := make([]uint64, num_rows)
	for row := uint64(0); row < num_rows; row++ {
		var vals []uint64
		// Recover each Z_p element that makes up the desired database entry
		for j := row * c.dbinfo.Ne; j < (row+1)*c.dbinfo.Ne; j++ {
			noised := ans.Get(j, 0) + offset
			denoised := c.params.Round(noised)
			vals = append(vals, denoised)
			//fmt.Printf("Reconstructing row %d: %d\n", j, denoised)
		}
		out[row] = ReconstructElem(vals, i, c.dbinfo)
		i += c.params.M
	}
	ans.Add(interm)

	return out
}

func (c *Client) RecoverManyLHE(secret *SecretLHE, ans *Answer) []uint64 {
	if (c.dbinfo.Packing != 1) || (c.dbinfo.Ne != 1) {
		panic("Not yet supported")
	}

	ratio := c.params.P / 2
	offset := uint64(0)
	for j := uint64(0); j < c.params.M; j++ {
		offset += ratio * secret.query.Get(j, 0)
	}
	offset %= (1 << c.params.Logq)
	offset = (1 << c.params.Logq) - offset

	interm := matrix.Mul(c.hint, secret.secret)
	ans.Sub(interm)

	norm := uint64(0)
	for _, elem := range secret.arr {
		norm += elem
	}

	out := make([]uint64, ans.Rows())
	for row := uint64(0); row < ans.Rows(); row++ {
		noised := ans.Get(row, 0) + offset
		denoised := c.params.Round(noised)
		out[row] = (denoised + ratio * norm) % c.params.P
	}
	ans.Add(interm)

	return out
}
