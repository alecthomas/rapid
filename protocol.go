package rapid

import (
	"encoding/json"
	"net/http"
)

// ErrorResponse is the wire-format for a RAPID error response.
type ErrorResponse struct {
	Error string `json:"e,omitempty" msgpack:"e"`
}

// Protocol contains various functions for assisting in translation between
// HTTP and the RAPID Go API.
type Protocol interface {
	ReadRequest(r *http.Request, v interface{}) error
	WriteResponse(r *http.Request, w http.ResponseWriter, status int, err error, data interface{})
	WriteRequest(v interface{}) (headers http.Header, body []byte, err error)
	ReadResponse(hr *http.Response, v interface{}) error
}

type DefaultProtocol struct{}

func (d *DefaultProtocol) ReadRequest(r *http.Request, v interface{}) error {
	// TODO: Parse Accept header, etc.
	return json.NewDecoder(r.Body).Decode(v)
}

func (d *DefaultProtocol) WriteResponse(r *http.Request, w http.ResponseWriter, status int, err error, data interface{}) {
	status, err = TranslateError(r, status, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err != nil {
		data = &ErrorResponse{Error: err.Error()}
	}
	_ = json.NewEncoder(w).Encode(data)
}

var contentTypeHeader = http.Header{"Content-Type": []string{"application/json"}, "Accept": []string{"application/json"}}

func (d *DefaultProtocol) WriteRequest(v interface{}) (http.Header, []byte, error) {
	data, err := json.Marshal(v)
	return contentTypeHeader, data, err
}

func (d *DefaultProtocol) ReadResponse(r *http.Response, v interface{}) error {
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		response := &ErrorResponse{}
		if err := json.NewDecoder(r.Body).Decode(response); err != nil {
			// Not a valid response structure, return error.
			return Error(http.StatusInternalServerError, err.Error())
		}
		// Use error in response structure.
		return Error(r.StatusCode, response.Error)
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
