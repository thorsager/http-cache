package httpCache

import (
	"io"
	"bytes"
	log "github.com/sirupsen/logrus"
	"os"
	"errors"
	"io/ioutil"
)


type hybridBufferWriter struct {
	MaxMemSize int
	memBuffer bytes.Buffer
	fileBuffer *os.File
	inFile bool
	writer io.Writer
	done bool
	filename string
}

func (hw *hybridBufferWriter) Write(d []byte) (n int, err error) {
	if hw.done {
		return 0,errors.New("writer after _close_")
	}
	if !hw.inFile && hw.memBuffer.Len() + len(d) > hw.MaxMemSize {
		hw.fileBuffer, err = ioutil.TempFile("","hbw")
		hw.filename = hw.fileBuffer.Name()
		if err != nil {
			return 0, err
		}
		log.Debugf("Create fileBuffer %s",hw.filename)

		n, err = hw.fileBuffer.Write(hw.memBuffer.Bytes())
		if err != nil {
			return 0, err
		}
		if n != hw.memBuffer.Len() {
			return 0, errors.New("failed to transition from memory to disk")
		}
		hw.memBuffer.Reset()
		hw.inFile = true
		log.Debugf("Wrote %d bytes from memBuffer to fileBuffer",n)
	}
	if !hw.inFile {
		//log.Debugf("Writing %d bytes to memBuffer (bufferSize=%d)",len(d),hw.memBuffer.Len())
		return hw.memBuffer.Write(d)
	} else {
		//log.Debugf("Writing %d bytes to fileBuffer (%s)",len(d),hw.filename)
		return hw.fileBuffer.Write(d)
	}
}

func (hw *hybridBufferWriter) ReadCloser() (reader io.ReadCloser, err error) {
	hw.done=true
	if hw.inFile {
		hw.fileBuffer.Sync()
		hw.fileBuffer.Close()
		hw.fileBuffer, err = os.Open(hw.filename)
		return hw.fileBuffer, err
	} else {
		return ioutil.NopCloser(bytes.NewReader(hw.memBuffer.Bytes())), nil
	}
}

func (hw *hybridBufferWriter) Close() error {
	if hw.inFile {
		hw.fileBuffer.Close()
		if err := os.Remove(hw.filename); err != nil {
			return err
		}
		log.Debugf("Removed file: %s", hw.filename)
	}
	return nil
}

type copyReadCloser struct {
	Reader      io.ReadCloser
	OnEof       func(io.Reader)
	Buffer 		hybridBufferWriter
}


// Please note that this implementation WILL retain a copy of
// the entire response-body in memory
func (r *copyReadCloser) Read(p []byte) (n int, err error) {
	n,err = r.copy(p)
	if err == io.EOF {
		rdr,err2 := r.Buffer.ReadCloser()
		if err2 != nil {
			return 0,err2
		}
		r.OnEof(rdr)
	}
	return n,err
}

func (r * copyReadCloser) copy(p []byte) (n int, err error) {
	n,err = r.Reader.Read(p)
	r.Buffer.Write(p[:n])
	return n,err
}

func (r *copyReadCloser) Close() error {
	r.Buffer.Close()
	return r.Reader.Close()
}