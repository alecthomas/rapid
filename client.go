package rapid

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"

	"github.com/alecthomas/rapid/schema"
)

type Client struct {
	url        string
	protocol   Protocol
	HTTPClient *http.Client
}

func Dial(url string, protocol Protocol) (*Client, error) {
	return &Client{
		url:        url,
		protocol:   protocol,
		HTTPClient: &http.Client{},
	}, nil
}

// Do issues a HTTP request to a rapid server.
// params are interpolated into path.
// query can be either nil, url.Values or a struct conforming to the
// gorilla/schema tag protocol.
func (c *Client) Do(method string, req, query interface{}, path string, params ...interface{}) (*http.Response, error) {
	path = schema.InterpolatePath(path, params...)
	var body io.Reader
	if req != nil {
		b := &bytes.Buffer{}
		if err := json.NewEncoder(b).Encode(req); err != nil {
			return nil, err
		}
		body = b
	}
	// Build URL.
	url := c.url + path
	q := schema.EncodeStructToURLValues(query)
	if len(q) > 0 {
		url += "?" + q.Encode()
	}
	hreq, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	hreq.Header.Add("Accept", "application/json")
	return c.HTTPClient.Do(hreq)
}

func (c *Client) DoBasic(method string, req, resp, query interface{}, path string, params ...interface{}) error {
	hr, err := c.Do(method, req, query, path, params...)
	if err != nil {
		return err
	}
	defer hr.Body.Close()
	if resp != nil {
		_, err = c.protocol.Decode(hr.Body, resp)
	}
	return err
}

func (c *Client) DoStreaming(method string, req, query interface{}, path string, params ...interface{}) (*ClientStream, error) {
	hr, err := c.Do(method, req, query, path, params...)
	if err != nil {
		return nil, err
	}
	return &ClientStream{hr: hr, r: httputil.NewChunkedReader(hr.Body), protocol: c.protocol}, nil
}

type Packet struct {
	Error error
	Data  []byte
}

type ClientStream struct {
	hr       *http.Response
	r        io.Reader
	protocol Protocol
}

func (c *ClientStream) Next(v interface{}) error {
	_, err := c.protocol.Decode(c.r, v)
	return err
}

func (c *ClientStream) Close() error {
	c.hr.Body.Close()
	return nil
}
