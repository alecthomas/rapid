package rapid

import (
	"errors"
	"net/http"
)

type Protocol interface {
	TranslateError(r *http.Request, status int, err error) (int, error)
}

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
