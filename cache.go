package httpCache

import "net/http"

type Cache interface {
	Key(r *http.Request) string
	Get(key string) (data []byte, found bool)
	Put(key string, data []byte) (ok bool)
	Delete(key string)
}
