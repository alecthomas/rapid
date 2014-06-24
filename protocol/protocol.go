// Package protocol implements a simple response encoding protocol for rapid.
package protocol

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Response is the response encoding structure.
type Response struct {
	S int         `json:"s"`
	E string      `json:"e,omitempty"`
	D interface{} `json:"d,omitempty"`
}

// DefaultProtocol implements a useful default API protocol.
type DefaultProtocol struct{}

func (d *DefaultProtocol) WriteHeader(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

func (d *DefaultProtocol) EncodeResponse(w http.ResponseWriter, r *http.Request, status int, err error, data interface{}) {
	var errString string
	if err != nil {
		errString = err.Error()
	}
	json.NewEncoder(w).Encode(&Response{S: status, E: errString, D: data})
}

func (d *DefaultProtocol) NotFound(w http.ResponseWriter, r *http.Request) {
	d.EncodeResponse(w, r, http.StatusNotFound, errors.New("not found"), nil)
}
