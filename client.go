package rapid

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type BeforeClientRequest func(*http.Request) error

type Client interface {
	BeforeRequest(hook BeforeClientRequest) error
	Do(req *RequestTemplate, resp interface{}) error
	Close() error
	HTTPClient() *http.Client
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
	codec   RequestResponseCodecFactory
	method  string
	path    string
	headers http.Header
	body    []byte
}

func (r *RequestTemplate) Build(url string) *http.Request {
	h, err := http.NewRequest(r.method, url+r.path, bytes.NewBuffer(r.body))
	if err != nil {
		panic(err)
	}
	h.Header = r.headers
	return h
}

func (r *RequestTemplate) String() string {
	return fmt.Sprintf("%s %s", r.method, r.path)
}

type RequestBuilder struct {
	codec    RequestResponseCodecFactory
	filename string
	method   string
	path     string
	query    interface{}
	body     interface{}
}

// Request makes a new RequestBuilder. A RequestBuilder is a type with useful
// constructs for building rapid-conformant HTTP requests.
// Parameters in the form {name} are interpolated into the path from params.
// eg. Request(codec, "GET", "/{id}", 10)
func Request(codec RequestResponseCodecFactory, method, path string, params ...interface{}) *RequestBuilder {
	if codec == nil {
		codec = DefaultCodecFactory
	}
	path = InterpolatePath(path, params...)
	return &RequestBuilder{
		codec:  codec,
		method: method,
		path:   path,
	}
}

// Query defines query parameters for a request. It accepts either url.Values
// or a struct conforming to gorilla/
func (r *RequestBuilder) Query(query interface{}) *RequestBuilder {
	r.query = query
	return r
}

// Body sets the JSON-encoded body of the request.
func (r *RequestBuilder) Body(v interface{}) *RequestBuilder {
	r.body = v
	return r
}

func (r *RequestBuilder) FileData(path string, data []byte) *RequestBuilder {
	r.filename = path
	r.body = ioutil.NopCloser(bytes.NewReader(data))
	return r
}

func (r *RequestBuilder) File(path string, rc io.ReadCloser) *RequestBuilder {
	r.filename = path
	r.body = rc
	return r
}

func (r *RequestBuilder) Data(data []byte) *RequestBuilder {
	r.body = ioutil.NopCloser(bytes.NewReader(data))
	return r
}

func (r *RequestBuilder) Build() *RequestTemplate {
	path := strings.TrimLeft(r.path, "/")
	q := EncodeStructToURLValues(r.query)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	headers, bodyr, err := MakeRequestCodec(r.body, r.codec).EncodeRequest()
	if err != nil {
		panic(err)
	}
	defer bodyr.Close()
	body, err := ioutil.ReadAll(bodyr)
	if err != nil {
		panic(err)
	}
	return &RequestTemplate{
		codec:   r.codec,
		method:  r.method,
		path:    path,
		headers: headers,
		body:    body,
	}
}

// A BasicClient is a simple client that issues one request per API call. It
// does not perform retries.
type BasicClient struct {
	codec      RequestResponseCodecFactory
	url        string
	beforeHook BeforeClientRequest
	httpClient *http.Client
}

// Dial creates a new RAPID client with url as its endpoint, using the given codec.
func Dial(codec RequestResponseCodecFactory, url string) (*BasicClient, error) {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return &BasicClient{
		codec:      codec,
		url:        url,
		httpClient: &http.Client{},
	}, nil
}

func (b *BasicClient) BeforeRequest(hook BeforeClientRequest) error {
	b.beforeHook = hook
	return nil
}

func (b *BasicClient) Do(req *RequestTemplate, resp interface{}) error {
	hr := req.Build(b.url)
	if b.beforeHook != nil {
		if err := b.beforeHook(hr); err != nil {
			return err
		}
	}
	response, err := b.httpClient.Do(hr)
	if err != nil {
		return err
	}
	return MakeResponseCodec(resp, b.codec).DecodeResponse(response)
}

func (b *BasicClient) HTTPClient() *http.Client {
	return b.httpClient
}

func (b *BasicClient) Close() error {
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
