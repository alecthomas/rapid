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

func (d *DefaultProtocol) EncodeResponse(w http.ResponseWriter, r *http.Request, code int, err error, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	var errString string
	if err != nil {
		errString = err.Error()
	}
	json.NewEncoder(w).Encode(&Response{S: code, E: errString, D: payload})
}

func (d *DefaultProtocol) NotFound(w http.ResponseWriter, r *http.Request) {
	d.EncodeResponse(w, r, http.StatusNotFound, errors.New("not found"), nil)
}
