package rapid

import (
	"encoding/json"
	"errors"
	"net/http"
)

// ProtocolReponse is the default protocol response encoding structure.
type ProtocolReponse struct {
	S int         `json:"S"`
	E string      `json:"E,omitempty"`
	D interface{} `json:"D,omitempty"`
}

// DefaultProtocol implements a useful default API protocol.
type DefaultProtocol struct{}

func (d *DefaultProtocol) TranslateError(err error) (int, error) {
	status := http.StatusOK
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
	json.NewEncoder(w).Encode(&ProtocolReponse{S: status, E: errString, D: data})
}

func (d *DefaultProtocol) NotFound(w http.ResponseWriter, r *http.Request) {
	d.EncodeResponse(w, r, http.StatusNotFound, errors.New("not found"), nil)
}
