package httpCache

// inspired VERY much by https://github.com/gregjones/httpcache/blob/master/httpcache.go

import (
	"bufio"
	"bytes"
	"errors"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"
)

// constants describing cached item state
const stale int = 0
const fresh int = 1
const transparent int = 2

type Transport struct {
	Cache     Cache
	Transport http.RoundTripper
}

func NewTransport(c Cache) *Transport {
	return &Transport{Cache: c}
}

func (t *Transport) Client() *http.Client {
	return &http.Client{Transport: t}
}

func (t *Transport) RoundTrip(req *http.Request) (resp *http.Response, err error) {
	cacheKey := t.Cache.Key(req)
	cacheable := (req.Method == "GET" || req.Method == "HEAD") && req.Header.Get("range") == ""
	rrtLog := log.WithFields(log.Fields{
		"key": cacheKey})

	var cachedResponse *http.Response
	if cacheable {
		cachedResponse, err = getCachedResponse(t.Cache, cacheKey, req)
	}

	transport := t.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	if cacheable && cachedResponse != nil && err == nil {
		cacheState := getCacheState(req.Header, cachedResponse.Header)

		if cacheState == fresh {
			rrtLog.Debugf("Returning fresh response")
			return cachedResponse, nil
		}
		if cacheState == stale {
			rrtLog.Debugf("Found Stale response for (refreshing)")
			// use cached etag
			etag := cachedResponse.Header.Get("etag")
			if etag != "" && req.Header.Get("etag") == "" {
				req.Header.Set("if-none-match", etag)
				rrtLog.Debugf("Using cached Etag=%s", etag)
			}
			// use cached last modified
			lastModified := cachedResponse.Header.Get("last-modified")
			if lastModified != "" && req.Header.Get("last-modified") == "" {
				req.Header.Set("if-modified-since", lastModified)
				rrtLog.Debugf("Using cached last-modified=%s", lastModified)
			}
		}
	}

	resp, err = transport.RoundTrip(req)

	if err == nil && cachedResponse != nil && resp.StatusCode == http.StatusNotModified {
		rrtLog.Debugf("Returning cached after NotModified")
		for _, header := range e2eHeaders(resp.Header) {
			rrtLog.Debugf("Header copy: %s=%s", header, resp.Header[header])
			cachedResponse.Header[header] = resp.Header[header]
		}
		resp = cachedResponse
	} else if err != nil || (cachedResponse != nil && resp.StatusCode >= 500) {
		// Todo implement  Stale on erro
	} else {
		if err != nil && resp.StatusCode != http.StatusOK {
			t.Cache.Delete(cacheKey)
		}
		if err != nil {
			return nil, err
		}

	}

	if err == nil && (resp.StatusCode > 199 && resp.StatusCode < 300) {
		cc := fetchCacheControl(resp.Header)
		if cacheable && canStore(fetchCacheControl(req.Header), cc) {
			rrtLog.Debugf("Can and will store (%v)", cc)
			switch req.Method {
			case http.MethodGet:
				rrtLog.Debug("Defer cache put until EOF")
				// This will have the "side-effect" that if the response
				// body is never read, it is also never cached.
				resp.Body = &copyReadCloser{
					Buffer: hybridBufferWriter{MaxMemSize: 50 * 1024},
					Reader: resp.Body,
					OnEof: func(r io.Reader) {
						resp := *resp
						resp.Body = ioutil.NopCloser(r)
						if respBytes, err := httputil.DumpResponse(&resp, true); err == nil {
							rrtLog.Debug("(deferred) Putting into cache")
							if ok := t.Cache.Put(cacheKey, respBytes); !ok {
								rrtLog.Debug("(deferred) Rejected by Cache")
							}
						}
					},
				}
			default:
				if respBytes, err := httputil.DumpResponse(resp, true); err == nil {
					rrtLog.Debug("Putting into cache")
					if accepted := t.Cache.Put(cacheKey, respBytes); !accepted {
						rrtLog.Debugf("Rejected by Cache")
					}
				} else {
					rrtLog.Error(err)
				}
			}
		}
	}
	return resp, err
}

func canStore(reqCC cacheControl, respCC cacheControl) bool {
	if _, ok := reqCC["no-store"]; ok {
		return false
	}
	if _, ok := respCC["no-store"]; ok {
		return false
	}
	return true
}

func e2eHeaders(respHeader http.Header) (e2eh []string) {
	hbhh := map[string]struct{}{
		"Connection":          {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"Te":                {},
		"Trailers":          {},
		"Transfer-Encoding": {},
		"Upgrade":           {},
	}
	for _, ch := range strings.Split(respHeader.Get("Connection"), ",") {
		if tch := strings.Trim(ch, " "); tch != "" {
			hbhh[http.CanonicalHeaderKey(tch)] = struct{}{}
		}
	}

	for header := range respHeader {
		if _, ok := hbhh[header]; !ok {
			e2eh = append(e2eh, header)
		}
	}
	return
}

func getCacheState(reqHeader http.Header, respHeader http.Header) (cacheState int) {
	reqCC := fetchCacheControl(reqHeader)
	respCC := fetchCacheControl(respHeader)

	if _, ok := reqCC["no-cache"]; ok {
		return transparent
	}
	if _, ok := respCC["no-cache"]; ok {
		return stale
	}
	if _, ok := reqCC["only-if-cached"]; ok {
		return fresh
	}

	rTime, err := fetchTime(respHeader)
	if err != nil {
		return stale
	}

	var ttl time.Duration
	var zeroTTL time.Duration

	rAge := time.Since(rTime)

	// Check Response for "max-age" and "Expires"
	if maxAge, ok := respCC["max-age"]; ok {
		ttl, err = time.ParseDuration(maxAge + "s")
		if err != nil {
			log.Warn("Error while parsing max-age (resp), ttl=zero")
			ttl = zeroTTL
		}
	} else {
		// Check for an Expires header
		if expiresHeader := respHeader.Get("Expires"); expiresHeader != "" {
			log.Debugf("Expires (resp): %s", expiresHeader)
			expires, err := rfc1123(expiresHeader)
			if err != nil {
				log.Warnf("Error while parsing Expires (%s), ttl=zero", expiresHeader)
				ttl = zeroTTL
			} else {
				ttl = expires.Sub(rTime)
			}
		}
	}

	if maxAge, ok := reqCC["max-age"]; ok {
		// Client is ok with receiving a response that i no older that the specified number of seconds.
		ttl, err = time.ParseDuration(maxAge + "s")
		if err != nil {
			log.Warn("Error while parsing req. max-age (%s), ttl=zero", maxAge)
			ttl = zeroTTL
		}
	}

	if minFresh, ok := reqCC["min-fresh"]; ok {
		// Client wants only responses that is still fresh for the specified number of seconds.
		minFreshDuration, err := time.ParseDuration(minFresh + "s")
		if err == nil {
			rAge = time.Duration(rAge - minFreshDuration)
		}
	}

	// TODO: implement "max-stale"

	log.Debugf("ttl vs. age : %d - %d", ttl, rAge)
	if ttl > rAge {
		return fresh
	}

	return stale
}

func getCachedResponse(c Cache, cacheKey string, r *http.Request) (resp *http.Response, err error) {
	data, ok := c.Get(cacheKey)
	if !ok {
		return nil, errors.New("key not found")
	}
	buf := bytes.NewBuffer(data)
	resp, err = http.ReadResponse(bufio.NewReader(buf), r)
	if err == nil {
		resp.Header.Add("X-Cached", "true")
	}
	return resp, err
}
