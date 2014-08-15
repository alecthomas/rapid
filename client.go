package rapid

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	"github.com/alecthomas/rapid/schema"
)

type BeforeClientRequest func(*http.Request) error

type Client interface {
	BeforeRequest(hook BeforeClientRequest) error
	Do(req *RequestTemplate, resp interface{}) error
	DoStreaming(req *RequestTemplate) (ClientStream, error)
	Close() error
}

type ClientStream interface {
	Next(v interface{}) error
	Close() error
}

// BasicAuthHook is a BeforeRequest hook for performing basic auth.
func BasicAuthHook(username, password string) BeforeClientRequest {
	return func(req *http.Request) error {
		req.SetBasicAuth(username, password)
		return nil
	}
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
	path := strings.TrimLeft(r.path, "/")
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
	beforeHook BeforeClientRequest
	HTTPClient *http.Client
}

// Dial creates a new RAPID client with url as its endpoint, using the given protocol.
func Dial(url string) (*BasicClient, error) {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return &BasicClient{
		url:        url,
		HTTPClient: &http.Client{},
	}, nil
}

func (b *BasicClient) do(req *RequestTemplate) (*http.Response, error) {
	hreq := req.Build(b.url)
	if b.beforeHook != nil {
		if err := b.beforeHook(hreq); err != nil {
			return nil, err
		}
	}
	hr, err := b.HTTPClient.Do(hreq)
	if err != nil {
		return nil, err
	}
	if hr.StatusCode < 200 || hr.StatusCode > 299 {
		defer hr.Body.Close()
		response := &intermediateProtocolResponse{}
		if err := json.NewDecoder(hr.Body).Decode(response); err != nil {
			// Not a valid response structure, return HTTP error.
			return nil, Error(hr.StatusCode, hr.Status)
		}
		// Use error in response structure.
		return nil, Error(response.Status, response.Error)
	}
	return hr, nil
}

func (b *BasicClient) BeforeRequest(hook BeforeClientRequest) error {
	b.beforeHook = hook
	return nil
}

func (b *BasicClient) Do(req *RequestTemplate, resp interface{}) error {
	hr, err := b.do(req)
	if err != nil {
		return err
	}
	defer hr.Body.Close()
	intr := &intermediateProtocolResponse{}
	err = json.NewDecoder(hr.Body).Decode(intr)
	if err != nil {
		return err
	}
	// No response value to decode into, just return.
	if resp == nil {
		return nil
	}
	// NOTE: We will never have an error response structure here, because do()
	// already handles error statuses.

	// We have a response value - decode wrapped data item into it.
	return json.Unmarshal(intr.Data, resp)
}

func (b *BasicClient) DoStreaming(req *RequestTemplate) (ClientStream, error) {
	hr, err := b.do(req)
	if err != nil {
		return nil, err
	}
	return &BasicClientStream{hr: hr, dec: json.NewDecoder(httputil.NewChunkedReader(hr.Body))}, nil
}

func (b *BasicClient) Close() error {
	return nil
}

type Packet struct {
	Error error
	Data  []byte
}

type BasicClientStream struct {
	hr  *http.Response
	dec *json.Decoder
}

func (b *BasicClientStream) Next(v interface{}) error {
	response := &intermediateProtocolResponse{}
	err := b.dec.Decode(response)
	if err != nil {
		return err
	}
	if response.Status < 200 || response.Status > 299 {
		return Error(response.Status, response.Error)
	}
	return json.Unmarshal(response.Data, v)
}

func (b *BasicClientStream) Close() error {
	b.hr.Body.Close()
	return nil
}

// type RetryingClient struct {
// 	client  Client
// 	backoff backoff.BackOff
// 	log     Logger
// }

// func NewRetryingClient(client Client, backoff backoff.BackOff, log Logger) (*RetryingClient, error) {
// 	backoff.Reset()
// 	return &RetryingClient{
// 		client:  client,
// 		backoff: backoff,
// 		log:     log,
// 	}, nil
// }

// func (r *RetryingClient) Do(req *RequestTemplate, resp interface{}) error {
// 	for {
// 		r.log.Debugf("Issing %s", req)
// 		err := r.client.Do(req, resp)
// 		if err == nil {
// 			return nil
// 		}

// 		duration := r.backoff.NextBackOff()
// 		r.log.Debugf("Request %s failed (%s), delaying for %s", req, err, duration)
// 		if duration == backoff.Stop {
// 			r.log.Debugf("Request %s exceeded retries, stopping", req)
// 			return err
// 		}
// 		time.Sleep(duration)
// 	}
// }

// func (r *RetryingClient) DoStreaming(req *RequestTemplate) (ClientStream, error) {
// 	for {
// 		r.log.Debugf("Issing streaming request to %s", req)
// 		stream, err := r.client.DoStreaming(req)
// 		if err == nil {
// 			return &RetryingClientStream{r.backoff, stream}, nil
// 		}

// 		duration := r.backoff.NextBackOff()
// 		r.log.Debugf("Streaming request %s failed (%s), delaying for %s", req, err, duration)
// 		if duration == backoff.Stop {
// 			r.log.Debugf("Streaming request %s exceeded retries, stopping", req)
// 			return nil, err
// 		}
// 		time.Sleep(duration)
// 	}
// }

// func (r *RetryingClient) Close() error {
// 	return r.client.Close()
// }

// type RetryingClientStream struct {
// 	backoff backoff.BackOff
// 	stream  ClientStream
// }

// func (r *RetryingClientStream) Next(v interface{}) error {
// 	for {
// 		err := r.stream.Next(v)
// 		if err == nil {
// 			return nil
// 		}

// 		duration := r.backoff.NextBackOff()
// 		if duration == backoff.Stop {
// 			return err
// 		}
// 		time.Sleep(duration)

// 	}
// }

// func (r *RetryingClientStream) Close() error {
// 	return r.stream.Close()
// }
