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
	WriteResponse(r *http.Request, w http.ResponseWriter, status int, err error, data interface{})
}

type DefaultProtocol struct{}

func (d *DefaultProtocol) translateError(r *http.Request, status int, err error) (int, error) {
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

func (d *DefaultProtocol) WriteResponse(r *http.Request, w http.ResponseWriter, status int, err error, data interface{}) {
	status, err = d.translateError(r, status, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	var response *ProtocolResponse
	if err != nil {
		response = &ProtocolResponse{
			Status: status,
			Error:  err.Error(),
		}
	} else {
		response = &ProtocolResponse{
			Status: status,
			Data:   data,
		}
	}
	_ = json.NewEncoder(w).Encode(response)
}
