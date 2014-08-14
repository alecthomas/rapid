package rapid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/cenkalti/backoff"

	"github.com/alecthomas/rapid/schema"
)

type Client interface {
	Do(req *RequestTemplate, resp interface{}) error
	DoStreaming(req *RequestTemplate) (ClientStream, error)
	Close() error
}

type ClientStream interface {
	Next(v interface{}) error
	Close() error
}

func MustClient(client Client, err error) Client {
	if err != nil {
		panic(err)
	}
	return client
}

// A RequestTemplate can be used to build a new http.Request from scratch.
type RequestTemplate struct {
	method string
	path   string
	body   *bytes.Buffer
}

func (r *RequestTemplate) Build(url string) *http.Request {
	h, err := http.NewRequest(r.method, url+r.path, r.body)
	if err != nil {
		panic(err)
	}
	if r.body != nil && r.body.Len() > 0 {
		h.Header.Set("Content-Type", "application/json")
	}
	return h
}

func (r *RequestTemplate) String() string {
	return fmt.Sprintf("%s %s", r.method, r.path)
}

type RequestBuilder struct {
	method string
	path   string
	query  interface{}
	body   interface{}
}

// Request makes a new RequestBuilder. A RequestBuilder is a type with useful
// constructs for building rapid-conformant HTTP requests.
// Parameters in the form {name} are interpolated into the path from params.
// eg. Request("GET", "/{id}", 10)
func Request(method, path string, params ...interface{}) *RequestBuilder {
	path = schema.InterpolatePath(path, params...)
	return &RequestBuilder{
		method: method,
		path:   path,
	}
}

// Query defines query parameters for a request. It accepts either url.Values
// or a struct conforming to gorilla/schema.
func (r *RequestBuilder) Query(query interface{}) *RequestBuilder {
	r.query = query
	return r
}

// Body sets the JSON-encoded body of the request.
func (r *RequestBuilder) Body(v interface{}) *RequestBuilder {
	r.body = v
	return r
}

func (r *RequestBuilder) Build() *RequestTemplate {
	path := r.path
	q := schema.EncodeStructToURLValues(r.query)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}

	// Encode request body, if any.
	body := bytes.NewBuffer(nil)
	if r.body != nil {
		if err := json.NewEncoder(body).Encode(r.body); err != nil {
			panic(err)
		}
	}

	return &RequestTemplate{
		method: r.method,
		path:   path,
		body:   body,
	}
}

// A BasicClient is a simple client that issues one request per API call. It
// does not perform retries.
type BasicClient struct {
	url        string
	protocol   Protocol
	HTTPClient *http.Client
}

// Dial creates a new RAPID client with url as its endpoint, using the given protocol.
func Dial(url string, protocol Protocol) (*BasicClient, error) {
	if protocol == nil {
		protocol = &DefaultProtocol{}
	}
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return &BasicClient{
		url:        url,
		protocol:   protocol,
		HTTPClient: &http.Client{},
	}, nil
}

func (c *BasicClient) Do(req *RequestTemplate, resp interface{}) error {
	hr, err := c.HTTPClient.Do(req.Build(c.url))
	if err != nil {
		return err
	}
	defer hr.Body.Close()
	if resp != nil {
		_, err = c.protocol.Decode(hr.Body, resp)
	}
	return err
}

func (c *BasicClient) DoStreaming(req *RequestTemplate) (ClientStream, error) {
	hr, err := c.HTTPClient.Do(req.Build(c.url))
	if err != nil {
		return nil, err
	}
	return &BasicClientStream{hr: hr, r: httputil.NewChunkedReader(hr.Body), protocol: c.protocol}, nil
}

func (c *BasicClient) Close() error {
	return nil
}

type Packet struct {
	Error error
	Data  []byte
}

type BasicClientStream struct {
	hr       *http.Response
	r        io.Reader
	protocol Protocol
}

func (c *BasicClientStream) Next(v interface{}) error {
	_, err := c.protocol.Decode(c.r, v)
	return err
}

func (c *BasicClientStream) Close() error {
	c.hr.Body.Close()
	return nil
}

type RetryingClient struct {
	client  Client
	backoff backoff.BackOff
	log     Logger
}

func NewRetryingClient(client Client, backoff backoff.BackOff, log Logger) (*RetryingClient, error) {
	backoff.Reset()
	return &RetryingClient{
		client:  client,
		backoff: backoff,
		log:     log,
	}, nil
}

func (r *RetryingClient) Do(req *RequestTemplate, resp interface{}) error {
	for {
		r.log.Debugf("Issing %s", req)
		err := r.client.Do(req, resp)
		if err == nil {
			return nil
		}

		duration := r.backoff.NextBackOff()
		r.log.Debugf("Request %s failed (%s), delaying for %s", req, err, duration)
		if duration == backoff.Stop {
			r.log.Debugf("Request %s exceeded retries, stopping", req)
			return err
		}
		time.Sleep(duration)
	}
}

func (r *RetryingClient) DoStreaming(req *RequestTemplate) (ClientStream, error) {
	for {
		r.log.Debugf("Issing streaming request to %s", req)
		stream, err := r.client.DoStreaming(req)
		if err == nil {
			return &RetryingClientStream{r.backoff, stream}, nil
		}

		duration := r.backoff.NextBackOff()
		r.log.Debugf("Streaming request %s failed (%s), delaying for %s", req, err, duration)
		if duration == backoff.Stop {
			r.log.Debugf("Streaming request %s exceeded retries, stopping", req)
			return nil, err
		}
		time.Sleep(duration)
	}
}

func (r *RetryingClient) Close() error {
	return r.client.Close()
}

type RetryingClientStream struct {
	backoff backoff.BackOff
	stream  ClientStream
}

func (r *RetryingClientStream) Next(v interface{}) error {
	for {
		err := r.stream.Next(v)
		if err == nil {
			return nil
		}

		duration := r.backoff.NextBackOff()
		if duration == backoff.Stop {
			return err
		}
		time.Sleep(duration)

	}
}

func (r *RetryingClientStream) Close() error {
	return r.stream.Close()
}
