package httpCache

import (
	"net/http"
	"strings"
	"time"
	"errors"
)

var NoDateHeader = errors.New("no date header found")

type cacheControl map[string]string

func fetchCacheControl(headers http.Header) cacheControl{
	control := cacheControl{}
	for _, item := range strings.Split(headers.Get("Cache-Control"),",") {
		item = strings.Trim(item," ")
		if item == "" { continue }
		if strings.ContainsRune(item,'=') {
			kv := strings.Split(item,"=")
			kv[0] = strings.Trim(kv[0]," ")
			kv[1] = strings.Trim(kv[1]," ")
			control[kv[0]]=kv[1]
		} else {
			control[item]=""
		}
	}
	return control
}

func rfc1123(rfc1123String string) (time.Time, error) {
	return time.Parse(time.RFC1123,rfc1123String)
}

func fetchTime(header http.Header) (responseTime time.Time, err error) {
	timeStr := header.Get("Date")

	if timeStr == "" {
		err = NoDateHeader; return
	}
	return rfc1123(timeStr)
}

