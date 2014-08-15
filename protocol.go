package rapid

import (
	"encoding/json"
	"net/http"
)

// ProtocolResponse is the wire-format for a RAPID response.
type ProtocolResponse struct {
	Status int         `json:"s"`
	Error  string      `json:"e,omitempty"`
	Data   interface{} `json:"d,omitempty"`
}

// Used during decoding to unwrap the framing ProtocolResponse structure.
type intermediateProtocolResponse struct {
	Status int             `json:"s"`
	Error  string          `json:"e,omitempty"`
	Data   json.RawMessage `json:"d,omitempty"`
}

// Protocol contains various functions for assisting in translation between
// HTTP and the RAPID Go API.
type Protocol interface {
	TranslateError(r *http.Request, status int, err error) (int, error)
}

type DefaultProtocol struct{}

func (d *DefaultProtocol) TranslateError(r *http.Request, status int, err error) (int, error) {
	// No error, just return status immediately.
	if err == nil {
		if status == 0 {
			if r.Method == "POST" {
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
