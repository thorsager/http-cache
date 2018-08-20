package httpCache

import (
	"io"
)

type copyReadCloser struct {
	Reader io.ReadCloser
	OnEof  func(io.Reader)
	Buffer hybridBufferWriter
}

// Please note that this implementation WILL retain a copy of
// the entire response-body in memory
func (r *copyReadCloser) Read(p []byte) (n int, err error) {
	n, err = r.copy(p)
	if err == io.EOF {
		rdr, err2 := r.Buffer.ReadCloser()
		if err2 != nil {
			return 0, err2
		}
		r.OnEof(rdr)
	}
	return n, err
}

func (r *copyReadCloser) copy(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	r.Buffer.Write(p[:n])
	return n, err
}

func (r *copyReadCloser) Close() error {
	r.Buffer.Close()
	return r.Reader.Close()
}
