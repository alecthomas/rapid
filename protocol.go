package rapid

import (
	"encoding/json"
	"net/http"
)

// ProtocolResponse is the wire-format for a RAPID response.
type ProtocolResponse struct {
	Status int         `json:"s" msgpack:"s"`
	Error  string      `json:"e,omitempty" msgpack:"e"`
	Data   interface{} `json:"d,omitempty" msgpack:"d"`
}

// Used during decoding to unwrap the framing ProtocolResponse structure.
type intermediateProtocolResponse struct {
	Status int             `json:"s" msgpack:"s"`
	Error  string          `json:"e,omitempty" msgpack:"e"`
	Data   json.RawMessage `json:"d,omitempty" msgpack:"d"`
}

// Protocol contains various functions for assisting in translation between
// HTTP and the RAPID Go API.
type Protocol interface {
	ContentType() string
	ReadRequest(r *http.Request, v interface{}) error
	WriteResponse(r *http.Request, w http.ResponseWriter, status int, err error, data interface{})
	WriteRequest(v interface{}) (body []byte, err error)
	ReadResponse(hr *http.Response, v interface{}) error
}

type DefaultProtocol struct{}

func (d *DefaultProtocol) ContentType() string {
	return "application/json"
}

func (d *DefaultProtocol) ReadRequest(r *http.Request, v interface{}) error {
	// TODO: Parse Accept header, etc.
	return json.NewDecoder(r.Body).Decode(v)
}

func (d *DefaultProtocol) WriteResponse(r *http.Request, w http.ResponseWriter, status int, err error, data interface{}) {
	status, err = TranslateError(r, status, err)
	w.Header().Set("Content-Type", d.ContentType())
	w.WriteHeader(status)
	if err != nil {
		data = &ProtocolResponse{Status: status, Error: err.Error()}
	}
	_ = json.NewEncoder(w).Encode(data)
}

func (d *DefaultProtocol) WriteRequest(v interface{}) ([]byte, error) {
	body, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return body, err
}

func (d *DefaultProtocol) ReadResponse(r *http.Response, v interface{}) error {
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		response := &intermediateProtocolResponse{}
		if err := json.NewDecoder(r.Body).Decode(response); err != nil {
			// Not a valid response structure, return error.
			return Error(http.StatusInternalServerError, err.Error())
		}
		// Use error in response structure.
		return Error(response.Status, response.Error)
	}
	return json.NewDecoder(r.Body).Decode(v)
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
		}
		return status, nil
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
