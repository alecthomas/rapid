package rapid

import (
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime"
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

type Codec interface {
	RequestCodec
	ResponseCodec
}

type panicResponseCodec struct {
	RequestCodec
}

// Convert a RequestCodec into a Codec with ResponseCodec methods that panic.
func NopResponseCodec(codec RequestCodec) Codec {
	return &panicResponseCodec{codec}
}

func (p *panicResponseCodec) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	panic("EncodeResponse() is not supported")
}
func (p *panicResponseCodec) DecodeResponse(r *http.Response) error {
	panic("DecodeResponse() is not supported")
}

type panicRequestCodec struct {
	ResponseCodec
}

// Convert a ResponseCodec into a Codec with RequestCodec methods that panic.
func NopRequestCodec(codec ResponseCodec) Codec {
	return &panicRequestCodec{codec}
}

func (p *panicRequestCodec) EncodeRequest() (headers http.Header, body io.ReadCloser, err error) {
	panic("EncodeRequest() is not supported")
}
func (p *panicRequestCodec) DecodeRequest(r *http.Request) error {
	panic("DecodeRequest() is not supported")
}

// ErrorResponse is the wire-format for a RAPID error response.
type ErrorResponse struct {
	Error string `json:"e,omitempty"`
}

// A CodecFactory is a function that
type CodecFactory func(v interface{}) Codec

// Return v if it conforms to RequestCodec, otherwise use CodecFactory to
// encode/decode v.
func (c CodecFactory) Request(v interface{}) RequestCodec {
	if c, ok := v.(RequestCodec); ok {
		return c
	}
	return c(v)
}

// Return v if it conforms to ResponseCodec, otherwise use CodecFactory to
// encode/decode v.
func (c CodecFactory) Response(v interface{}) ResponseCodec {
	if c, ok := v.(ResponseCodec); ok {
		return c
	}
	return c(v)
}

// Return v if it conforms to Codec, otherwise use CodecFactory to encode/decode v.
func (c CodecFactory) Codec(v interface{}) Codec {
	if c, ok := v.(Codec); ok {
		return c
	}
	return c(v)
}

// Default, JSON codec.
type defaultCodec struct {
	v interface{}
}

// Create a default JSON Codec.
func DefaultCodecFactory(v interface{}) Codec {
	return &codecWrapper{&defaultCodec{v}}
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
	return json.NewDecoder(r.Body).Decode(d.v)
}

func (d *defaultCodec) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	data := d.v
	if err != nil && (status < 200 || status > 299) {
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

type FileDownload struct {
	Filename  string
	MediaType string
	Reader    io.ReadCloser
}

func (f *FileDownload) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	h := w.Header()
	h.Set("Content-Type", f.MediaType)
	h.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": f.Filename,
	}))
	w.WriteHeader(status)
	_, err = io.Copy(w, f.Reader)
	return err
}

func (f *FileDownload) DecodeResponse(r *http.Response) error {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition"))
	if err != nil {
		return err
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	f.MediaType = mt
	f.Filename = params["filename"]
	f.Reader = r.Body
	return nil
}

type FileUpload struct {
	Filename  string
	MediaType string
	Reader    io.ReadCloser
}

func (f *FileUpload) EncodeRequest() (headers http.Header, body io.ReadCloser, err error) {
	headers = http.Header{}
	headers.Set("Content-Type", f.MediaType)
	headers.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": f.Filename,
	}))
	return headers, f.Reader, nil
}

func (f *FileUpload) DecodeRequest(r *http.Request) error {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Disposition"))
	if err != nil {
		return err
	}
	mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return err
	}
	f.MediaType = mt
	f.Filename = params["filename"]
	f.Reader = r.Body
	return nil
}

type RawData []byte

func (d *RawData) EncodeRequest() (headers http.Header, body io.ReadCloser, err error) {
	headers = http.Header{}
	headers.Add("Content-Type", "application/octet-stream")
	return headers, ioutil.NopCloser(bytes.NewReader(*d)), nil
}

func (d *RawData) DecodeRequest(r *http.Request) error {
	defer r.Body.Close()
	w := bytes.NewBuffer(nil)
	if _, err := io.Copy(w, r.Body); err != nil {
		return err
	}
	*d = w.Bytes()
	return nil
}

func (d *RawData) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	_, e := io.Copy(w, bytes.NewReader(*d))
	return e
}

func (d *RawData) DecodeResponse(r *http.Response) error {
	defer r.Body.Close()
	w := bytes.NewBuffer(nil)
	if _, err := io.Copy(w, r.Body); err != nil {
		return err
	}
	*d = w.Bytes()
	return nil
}

type codecWrapper struct {
	Codec
}

// Wrapper factory providing some Codec convenience operations, such as
// automatic status+error inference, and injection of headers from HTTPStatus
// error values.
func NewResponseFixupCodecFactory(codec CodecFactory) CodecFactory {
	return func(v interface{}) Codec { return &codecWrapper{codec(v)} }
}

func (e *codecWrapper) EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error {
	status, err = FixupResonse(r, w, status, err)
	return e.Codec.EncodeResponse(r, w, status, err)
}

// inferStatus attempts to infer the response status and error.
// If err and status are zero, status will be 201 for a POST response
// and 200 for any other method.
// For any other non-zero status when err is nil, err will be set to
// an HTTPError instance.
// If err is already a HTTPStatus, we extract the status code.
func inferStatus(r *http.Request, status int, err error) (int, error) {
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
	} else if status == 0 {
		// If it's any other error type, set 500 and continue.
		status = http.StatusInternalServerError
		err = ErrorForStatus(status)
	}
	return status, err
}

// Convenience function that infers HTTP status and error, and injects headers
// from HTTPStatus error values (typically produced by Error(),
// ErrorForStatus() or ErrorWithHeaders()).
// Also see NewResponseFixupCodecFactory(), which creates a Codec that provides
// this functionality.
func FixupResonse(r *http.Request, w http.ResponseWriter, status int, err error) (int, error) {
	status, err = inferStatus(r, status, err)
	if e, ok := err.(*HTTPStatus); ok {
		if e.Headers != nil {
			for key, values := range e.Headers {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
		}
	}
	// If it's not an error, clear it so we don't return an unexpected error.
	if status >= 200 && status <= 299 {
		err = nil
	}
	return status, err
}
