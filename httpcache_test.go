package httpCache

import (
	"net/http"
	"github.com/thorsager/4chan/httpCache/memCache"
	"testing"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
)

var c *http.Client
var cache *memCache.MemCache

func init() {
	log.SetLevel(log.DebugLevel)
}

func setupFunc() func() {
	cache = &memCache.MemCache{MaxItemSize:50*1025}
	cache.Init()
	c = NewTransport(cache).Client()
	return func() {
		defer cache.Close()
	}
}

func Test_SimpleGET(t *testing.T) {
	tearDown := setupFunc()
	defer tearDown()

	const url = "http://myip.dk/img/logo3.png"
	req, err := http.NewRequest("GET",url,nil)
	if err != nil {
		t.Errorf("Fail to create GET Request on %s : %+v",url, err)
	}
	resp, err := c.Do(req)
	defer resp.Body.Close()
	if err != nil {
		t.Errorf("Fail to Do GET-request on %s : %+v",url,err)
	}

	if val := resp.Header.Get("X-Cached"); val != "" {
		t.Error("First Request CANNOT be from Cache")
	}

	if _,err = ioutil.ReadAll(resp.Body); err != nil {
		t.Error("Unable to reade response-body")
	}

	t.Logf("%d %s (%s)",resp.StatusCode, resp.Status,req.URL)

	resp2, err := c.Do(req)
	if err != nil {
		t.Errorf("Fail to Do GET-request on %s : %+v",url,err)
	}

	if val := resp2.Header.Get("X-Cached"); val == "" {
		t.Error("2nd Request SHOULD be from Cache")
	}
	if _,err = ioutil.ReadAll(resp2.Body); err != nil {
		t.Error("Unable to reade response-body")
	}



}

