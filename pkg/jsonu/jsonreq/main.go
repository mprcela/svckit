// Doing retryable json request.
package jsonreq

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

const (
	defaultRetries   = 10
	maxRetrySleep    = 10
	versionHeaderKey = "X-M5-Version"
)

type request struct {
	url        string
	body       []byte
	retries    int
	retrySleep int
	headers    http.Header
	method     string
	rsp        *http.Response
}

//Retries set number of retries and max delay between them
//delay will be exponentialy incresed from 1 to max
func Retries(retries, maxRetrySleepSec int) func(*request) {
	return func(r *request) {
		r.retries = retries
		r.retrySleep = maxRetrySleepSec
	}
}

//Header set header for request
func Header(key, value string) func(*request) {
	return func(r *request) {
		if r.headers == nil {
			r.headers = http.Header{}
		}
		r.headers.Set(key, value)
	}
}

//VersionHeader set version header for request
func VersionHeader(value string) func(*request) {
	return Header(versionHeaderKey, value)
}

//GetWithVersion - make Get request return body and version header
func GetWithVersion(url string, version string) ([]byte, string, error) {
	r := New(url, VersionHeader(version))
	rsp, err := r.Get()
	if err != nil {
		return rsp, version, err
	}
	version = r.VersionHeader()
	return rsp, version, nil
}

//Header read response header
func (r *request) Header(key string) string {
	if r.rsp == nil {
		return ""
	}
	return r.rsp.Header.Get(key)
}

//VersionHeader read response version header
func (r *request) VersionHeader() string {
	if r.rsp == nil {
		return ""
	}
	return r.rsp.Header.Get(versionHeaderKey)
}

func (r *request) StatusCode() int {
	if r.rsp == nil {
		return 0
	}
	return r.rsp.StatusCode
}

//New new request on url with optional options
//Example:
//  r := New("http://localhost:8091/")
//  rsp, err = r.Get()
//
//Setting options:
//  r := New("http://localhost:8091/",	Header("pero", "zdero"))
//  rsp, err = r.Post(data)
//
func New(url string, options ...func(*request)) *request {
	r := &request{
		url:        url,
		retries:    defaultRetries,
		retrySleep: maxRetrySleep,
		method:     "POST",
	}
	//apply options
	for _, o := range options {
		o(r)
	}
	return r
}

//Get make get request
func (r *request) Get() ([]byte, error) {
	r.method = "GET"
	return r.do()
}

//Post make post request
func (r *request) Post(body []byte) ([]byte, error) {
	r.body = body
	r.method = "POST"
	return r.do()
}

func (r *request) do() ([]byte, error) {
	var rsp []byte
	var err error
	for retry := 0; retry < r.retries; retry++ {
		var retryable bool
		rsp, err, retryable = r.one()
		if err == nil || !retryable {
			break
		} else {
			retryAfter := r.calcRetryInterval(retry)
			log.Printf("error: %s, will retry in %d seconds, retry no %d/%d", err, retryAfter, retry+1, r.retries)
			time.Sleep(time.Duration(1e9 * retryAfter))
		}
	}
	return rsp, err
}

func (r *request) one() ([]byte, error, bool) {
	log.Printf("[dubug] http.NewRequest(%s, %s)", r.method, r.url)
	req, err := http.NewRequest(r.method, r.url, bytes.NewReader(r.body))
	if err != nil {
		return nil, err, true
	}
	log.Printf("[dubug] req.URL %s %s", req.URL.Host, req.URL.Path)
	if r.headers != nil {
		r.copyHeader(req.Header, r.headers)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.ContentLength = int64(len(r.body))
	rsp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err, true
	}
	defer rsp.Body.Close()
	r.rsp = rsp
	//non retryable errors
	if rsp.StatusCode >= 300 && rsp.StatusCode < 500 {
		return nil, fmt.Errorf("%s response status code: %d", r.url, rsp.StatusCode), false
	}
	//retryable errors
	if rsp.StatusCode < 200 || rsp.StatusCode >= 500 {
		return nil, fmt.Errorf("%s response status code: %d", r.url, rsp.StatusCode), true
	}

	rspBody, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return nil, err, true
	}

	return rspBody, nil, true
}

func (r *request) calcRetryInterval(retry int) int {
	retryAfter := int(math.Exp(float64(retry)))
	if retryAfter > r.retrySleep {
		retryAfter = r.retrySleep
	}
	return retryAfter
}

func (r *request) copyHeader(dst, src http.Header) {
	for k, vv := range src {
		if !strings.Contains(k, "Accept-Encoding") {
			for _, v := range vv {
				dst.Add(k, v)
			}
		}
	}
}
