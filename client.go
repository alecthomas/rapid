package rapid

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/alecthomas/rapid/schema"
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
	protocol Protocol
	method   string
	path     string
	headers  http.Header
	body     []byte
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
	protocol Protocol
	filename string
	method   string
	path     string
	query    interface{}
	body     interface{}
}

// Request makes a new RequestBuilder. A RequestBuilder is a type with useful
// constructs for building rapid-conformant HTTP requests.
// Parameters in the form {name} are interpolated into the path from params.
// eg. Request(protocol, "GET", "/{id}", 10)
func Request(protocol Protocol, method, path string, params ...interface{}) *RequestBuilder {
	if protocol == nil {
		protocol = &DefaultProtocol{}
	}
	path = schema.InterpolatePath(path, params...)
	return &RequestBuilder{
		protocol: protocol,
		method:   method,
		path:     path,
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

func (r *RequestBuilder) Build() *RequestTemplate {
	path := strings.TrimLeft(r.path, "/")
	q := schema.EncodeStructToURLValues(r.query)
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	var body []byte
	var headers http.Header
	var err error
	if r.filename != "" {
		body, headers = r.makeFileUpload()
	} else {
		headers, body, err = r.protocol.WriteRequest(r.body)
		if err != nil {
			panic(err)
		}
	}
	return &RequestTemplate{
		protocol: r.protocol,
		method:   r.method,
		path:     path,
		headers:  headers,
		body:     body,
	}
}

func (r *RequestBuilder) makeFileUpload() ([]byte, http.Header) {
	in := r.body.(io.ReadCloser)
	defer in.Close()
	out := &bytes.Buffer{}
	w := multipart.NewWriter(out)
	defer w.Close()
	part, err := w.CreateFormFile("file", filepath.Base(r.path))
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(part, in)
	if err != nil {
		panic(err)
	}
	headers := http.Header{}
	headers.Add("Content-Type", w.FormDataContentType())
	return out.Bytes(), headers
}

// A BasicClient is a simple client that issues one request per API call. It
// does not perform retries.
type BasicClient struct {
	protocol   Protocol
	url        string
	beforeHook BeforeClientRequest
	httpClient *http.Client
}

// Dial creates a new RAPID client with url as its endpoint, using the given protocol.
func Dial(protocol Protocol, url string) (*BasicClient, error) {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return &BasicClient{
		protocol:   protocol,
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
	return b.protocol.ReadResponse(response, resp)
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
