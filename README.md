http-cache
==========
This is not a project ever meant for production, it is some-thing I am writing
to lean [Go](https://golang.org), I got some of the basic stuff form 
[httpcache](https://github.com/gregjones/httpcache/blob/master/httpcache.go).
I have tried to put my own spin on it, but thanks to [gregjones's](https://github.com/gregjones/)
for the inspiration 

```go
package main

import (
	"github.com/thorsager/http-cache/memCache"
	"github.com/thorsager/http-cache"
	"bytes"
	"fmt"
	log "github.com/sirupsen/logrus"
)
func init() {
	log.SetLevel(log.DebugLevel)
}
func main() {
	cache := memCache.New(50*1024*1024) // only store items less than 50K in size
	defer cache.Close()

	cachedTransport := httpCache.NewTransport(cache)
	httpClient := cachedTransport.Client()
	// Or you could do
	//httpClient := &http.Client{Transport:cachedTransport}

	resp, err := httpClient.Get("https://www.google.com")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	body := new(bytes.Buffer)
	body.ReadFrom(resp.Body)
	fmt.Print(body.String())
}
```
