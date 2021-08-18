package main

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/talon-one/go-httphandler"
)

func main() {
	http.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		if r.Method != http.MethodPost {
			// return with bad request if the method is not POST
			// respond with the clients Accept header content type
			return &httphandler.HandlerError{
				StatusCode:    http.StatusBadRequest,
				PublicError:   errors.New("only POST method is allowed"),
				InternalError: nil,
				ContentType:   "",
			}
		}

		if r.Body == nil {
			// return with internal server error if body is not available
			// respond with the clients Accept header content type
			return &httphandler.HandlerError{
				InternalError: errors.New("body was nil"),
			}
		}
		if _, err := ioutil.ReadAll(r.Body); err != nil {
			return &httphandler.HandlerError{
				InternalError: err,
			}
		}

		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, "ok"); err != nil {
			return &httphandler.HandlerError{
				InternalError: err,
			}
		}
		return nil
	}))
	http.ListenAndServe(":8000", nil)
}
