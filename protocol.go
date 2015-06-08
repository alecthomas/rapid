package rapid

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
)

// Encoding and decoding requests on the client and server, respectively.
type RequestCodec interface {
	// Encode request on client.
	EncodeRequest() (headers http.Header, body io.ReadCloser, err error)
	// Decode request.
	DecodeRequest(r *http.Request) error
}

// Encoding and decoding responses on the server and client, respectively.
type ResponseCodec interface {
	// Encode response into w.
	// "http.Request" is included to support Accept-based responses.
	EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error
	// Decode response from r.
	DecodeResponse(r *http.Response) error
}

type RequestResponseCodec interface {
	RequestCodec
	ResponseCodec
}

// ErrorResponse is the wire-format for a RAPID error response.
type ErrorResponse struct {
	Error string `json:"e,omitempty"`
}

// Default, JSON codec.
type defaultCodec struct {
	v interface{}
}

type RequestResponseCodecFactory func(v interface{}) RequestResponseCodec

func MakeRequestCodec(v interface{}, factory RequestResponseCodecFactory) RequestCodec {
	if c, ok := v.(RequestCodec); ok {
		return c
	}
	return factory(v)
}

func MakeResponseCodec(v interface{}, factory RequestResponseCodecFactory) ResponseCodec {
	if c, ok := v.(ResponseCodec); ok {
		return c
	}
	return factory(v)
}

func MakeRequestResponseCodec(v interface{}, factory RequestResponseCodecFactory) RequestResponseCodec {
	if c, ok := v.(RequestResponseCodec); ok {
		return c
	}
	return factory(v)
}

func DefaultCodecFactory(v interface{}) RequestResponseCodec {
	return &defaultCodec{v}
}

var contentTypeHeader = http.Header{"Content-Type": {"application/json"}, "Accept": {"application/json"}}

func (d *defaultCodec) EncodeRequest() (http.Header, io.ReadCloser, error) {
	body, err := json.Marshal(d.v)
	if err != nil {
		return nil, nil, err
	}
	return contentTypeHeader, ioutil.NopCloser(bytes.NewReader(body)), nil
}

func (d *defaultCodec) DecodeRequest(r *http.Request) error {
	// TODO: Parse Accept header, etc.
	return json.NewDecoder(r.Body).Decode(d.v)
}

func (d *defaultCodec) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	status, err = TranslateError(r, status, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data := d.v
	if err != nil {
		data = &ErrorResponse{Error: err.Error()}
	}
	return json.NewEncoder(w).Encode(data)
}

func (d *defaultCodec) DecodeResponse(r *http.Response) error {
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		response := &ErrorResponse{}
		if err := json.NewDecoder(r.Body).Decode(response); err != nil {
			// Not a valid response structure, return error.
			return Error(http.StatusInternalServerError, err.Error())
		}
		// Use error in response structure.
		return Error(r.StatusCode, response.Error)
	}
	return json.NewDecoder(r.Body).Decode(d.v)
}

func TranslateError(r *http.Request, status int, err error) (int, error) {
	// No error, just return status immediately.
	if err == nil {
		if status == 0 {
			if r != nil && r.Method == "POST" {
				status = http.StatusCreated
			} else {
				status = http.StatusOK
			}
		} else if status < 200 || status > 299 {
			err = ErrorForStatus(status)
		}
		return status, err
	}

	// Check if it's a HTTPStatus error, in which case check the status code.
	if st, ok := err.(*HTTPStatus); ok {
		status = st.Status
		// If it's not an error, clear the error value so we don't return an error value.
		if st.Status >= 200 && st.Status <= 299 {
			err = nil
		}
	} else if status == 0 {
		// If it's any other error type, set 500 and continue.
		status = http.StatusInternalServerError
	}
	return status, err
}
