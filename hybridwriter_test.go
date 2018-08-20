package httpCache

import (
	"io/ioutil"
	"testing"
)

func TestHybridWriter_Write(t *testing.T) {
	hr := hybridBufferWriter{MaxMemSize: 150}
	defer hr.Close()

	chunk := []byte("This_this_a_chunk_of_chars")
	for i := 0; i < 10; i++ {
		hr.Write(chunk)
	}
	reader, err := hr.ReadCloser()
	if err != nil {
		t.Errorf("Unable to get Reader: %v", err)
	}
	defer reader.Close()

	d, err := ioutil.ReadAll(reader)
	if err != nil {
		t.Errorf("Reader Should not Fail: %v", err)
	}
	if len(d) != len(chunk)*10 {
		t.Errorf("Size Missmatch %d vs %d", len(d), len(chunk)*10)
	}
}
