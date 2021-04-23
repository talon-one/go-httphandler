# go-httphandler [![Actions Status](https://github.com/talon-one/go-httphandler/workflows/CI/badge.svg)](https://github.com/talon-one/go-httphandler/actions) [![Coverage Status](https://coveralls.io/repos/github/talon-one/go-httphandler/badge.svg?branch=master)](https://coveralls.io/github/talon-one/go-httphandler?branch=master) [![PkgGoDev](https://img.shields.io/badge/pkg.go.dev-reference-blue)](https://pkg.go.dev/github.com/talon-one/go-httphandler) [![GoDoc](https://godoc.org/github.com/talon-one/go-httphandler?status.svg)](https://godoc.org/github.com/talon-one/go-httphandler) [![go-report](https://goreportcard.com/badge/github.com/talon-one/go-httphandler)](https://goreportcard.com/report/github.com/talon-one/go-httphandler)

*go-httphandler* is an http middleware that can be used to simplify the error response.
In case of error *go-httphandler* renders the error based on its Content-Type, or (if its not set) based on the clients
`Accept` header.

> go get -u github.com/talon-one/go-httphandler

## Example

```go
package main

import (
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
				PublicError:   "only POST method is allowed",
				InternalError: nil,
				ContentType:   "",
			}
		}

		if r.Body == nil {
			// return with internal server error if body is not available
			// respond with the clients Accept header content type
			return &httphandler.HandlerError{
				InternalError: "body was nil",
			}
		}
		if _, err := ioutil.ReadAll(r.Body); err != nil {
			return &httphandler.HandlerError{
				InternalError: err.Error(),
			}
		}

		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, "ok"); err != nil {
			return &httphandler.HandlerError{
				InternalError: err.Error(),
			}
		}
		return nil
	}))
	http.ListenAndServe(":8000", nil)
}
```
