package rapid

import (
	"encoding/json"
	"errors"
	"net/http"
)

// Protocol defining how responses and errors are encoded.
type Protocol interface {
	// TranslateError can be used to translate errors into rapid.HTTPStatus values.
	// in may be nil. status may be 0.
	TranslateError(r *http.Request, inStatus int, inError error) (status int, out error)
	WriteHeader(w http.ResponseWriter, r *http.Request, status int)
	EncodeResponse(w http.ResponseWriter, r *http.Request, status int, err error, data interface{})
	NotFound(w http.ResponseWriter, r *http.Request)
}

// ProtocolReponse is the default protocol response encoding structure.
type ProtocolReponse struct {
	S int         `json:"S"`
	E string      `json:"E,omitempty"`
	D interface{} `json:"D,omitempty"`
}

// DefaultProtocol implements a useful default API protocol.
type DefaultProtocol struct{}

func (d *DefaultProtocol) TranslateError(r *http.Request, status int, err error) (int, error) {
	if status == 0 {
		if r.Method == "POST" {
			status = http.StatusCreated
		} else {
			status = http.StatusOK
		}
	}

	if err == nil {
		return status, nil
	}

	// Check if it's a HTTPStatus error, in which case check the status code.
	if st, ok := err.(*HTTPStatus); ok {
		status = st.Status
		// If it's not an error, clear the error value so we don't return an error value.
		if st.Status >= 200 && st.Status <= 299 {
			err = nil
		}
	} else {
		// If it's any other error type, set 500 and continue.
		status = http.StatusInternalServerError
		err = errors.New("internal server error")
	}
	return status, err
}

func (d *DefaultProtocol) WriteHeader(w http.ResponseWriter, r *http.Request, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
}

func (d *DefaultProtocol) EncodeResponse(w http.ResponseWriter, r *http.Request, status int, err error, data interface{}) {
	var errString string
	if err != nil {
		errString = err.Error()
	}
	_ = json.NewEncoder(w).Encode(&ProtocolReponse{S: status, E: errString, D: data})
}

func (d *DefaultProtocol) NotFound(w http.ResponseWriter, r *http.Request) {
	d.EncodeResponse(w, r, http.StatusNotFound, errors.New("not found"), nil)
}
