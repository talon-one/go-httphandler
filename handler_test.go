package httphandler_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Eun/go-hit"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/require"

	"github.com/talon-one/go-httphandler"
)

func TestHandleFunc(t *testing.T) {
	options := httphandler.Options{
		LogFunc: func(handlerError error, internalError, publicError interface{}, statusCode int, requestUUID string) {
			require.EqualError(t, handlerError, "handler error")
			require.Nil(t, internalError)
			require.Equal(t, "bad request", publicError)
			require.Equal(t, http.StatusBadRequest, statusCode)
			require.Equal(t, "0123456789", requestUUID)
		},
		Encoders: map[string]httphandler.EncodeFunc{
			"text/html": func(w http.ResponseWriter, r *http.Request, _ *httphandler.WireError) error {
				require.Equal(t, "0123456789", httphandler.GetRequestUUID(r))
				_, err := io.WriteString(w, "html")
				return err
			},
			"application/json": func(w http.ResponseWriter, r *http.Request, _ *httphandler.WireError) error {
				require.Equal(t, "0123456789", httphandler.GetRequestUUID(r))
				_, err := io.WriteString(w, "json")
				return err
			},
		},
		FallbackEncoderFunc: func() (httphandler.EncodeFunc, string) {
			return func(w http.ResponseWriter, r *http.Request, _ *httphandler.WireError) error {
				require.Equal(t, "0123456789", httphandler.GetRequestUUID(r))
				_, err := io.WriteString(w, "fallback")
				return err
			}, "application/octet-stream"
		},
		RequestUUIDFunc: func() string {
			return "0123456789"
		},
	}

	h := httphandler.New(&options)
	mux := http.NewServeMux()
	mux.HandleFunc("/no-content-type-set", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		if r.Method != http.MethodPost {
			return &httphandler.HandlerError{
				StatusCode:  http.StatusBadRequest,
				PublicError: "bad request",
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}))
	mux.HandleFunc("/text/html", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		if r.Method != http.MethodPost {
			return &httphandler.HandlerError{
				StatusCode:  http.StatusBadRequest,
				PublicError: "bad request",
				ContentType: "text/html",
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}))
	mux.HandleFunc("/application/json", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		if r.Method != http.MethodPost {
			return &httphandler.HandlerError{
				StatusCode:  http.StatusBadRequest,
				PublicError: "bad request",
				ContentType: "application/json",
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return nil
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	expectFallback := hit.CombineSteps(
		hit.Expect().Status().Equal(http.StatusBadRequest),
		hit.Expect().Headers("Content-Type").Equal("application/octet-stream"),
		hit.Expect().Body().String().Equal("fallback"),
	)

	expectHMTL := hit.CombineSteps(
		hit.Expect().Status().Equal(http.StatusBadRequest),
		hit.Expect().Headers("Content-Type").Equal("text/html"),
		hit.Expect().Body().String().Equal("html"),
	)
	expectJSON := hit.CombineSteps(
		hit.Expect().Status().Equal(http.StatusBadRequest),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().String().Equal("json"),
	)

	t.Run("no accept header set", func(t *testing.T) {
		hit.Test(t,
			hit.Description("no-content-type-set endpoint should respond with application/octet-stream"),
			hit.Get(hit.JoinURL(s.URL, "no-content-type-set")),
			expectFallback,
		)
		hit.Test(t,
			hit.Description("text/html endpoint should respond with text/html"),
			hit.Get(hit.JoinURL(s.URL, "text/html")),
			expectHMTL,
		)
		hit.Test(t,
			hit.Description("application/json endpoint should respond with application/json"),
			hit.Get(hit.JoinURL(s.URL, "application/json")),
			expectJSON,
		)
	})

	t.Run("accept header set", func(t *testing.T) {
		hit.Test(t,
			hit.Description("no-content-type-set endpoint should respond with text/html"),
			hit.Get(hit.JoinURL(s.URL, "no-content-type-set")),
			hit.Send().Headers("Accept").Add("text/html"),
			expectHMTL,
		)
		hit.Test(t,
			hit.Description("text/html endpoint should respond with text/html"),
			hit.Get(hit.JoinURL(s.URL, "text/html")),
			hit.Send().Headers("Accept").Add("text/html"),
			expectHMTL,
		)
		hit.Test(t,
			hit.Description("application/json endpoint should respond with application/json"),
			hit.Get(hit.JoinURL(s.URL, "application/json")),
			hit.Send().Headers("Accept").Add("text/html"),
			expectJSON,
		)
	})

	t.Run("no error", func(t *testing.T) {
		hit.Test(t,
			hit.Post(hit.JoinURL(s.URL, "application/json")),
			hit.Expect().Status().Equal(http.StatusNoContent),
		)
	})
}

func TestInvalidAcceptHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{
			StatusCode: http.StatusInternalServerError,
		}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Send().Headers("Accept").Add(""),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
	)
}

func TestDefaultErrorValues(t *testing.T) {
	h := httphandler.New(nil)
	require.NoError(t, h.SetRequestUUIDFunc(func() string {
		return "0123456789"
	}))
	require.NoError(t, h.SetLogFunc(func(handlerError error, internalError, publicError interface{}, statusCode int, requestUUID string) {
		require.EqualError(t, handlerError, "handler error")
		require.Nil(t, internalError)
		require.Equal(t, "unknown error", publicError)
		require.Equal(t, http.StatusInternalServerError, statusCode)
		require.Equal(t, "0123456789", requestUUID)
	}))
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
		hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
		hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
	)
}

func TestFaultyEncoder(t *testing.T) {
	var errorLog []error
	options := httphandler.Options{
		LogFunc: func(handlerError error, internalError, publicError interface{}, statusCode int, requestUUID string) {
			errorLog = append(errorLog, handlerError)
		},
		Encoders: map[string]httphandler.EncodeFunc{},
		FallbackEncoderFunc: func() (httphandler.EncodeFunc, string) {
			return func(w http.ResponseWriter, r *http.Request, _ *httphandler.WireError) error {
				return errors.New("encoder error")
			}, "application/octet-stream"
		},
		RequestUUIDFunc: func() string {
			return "0123456789"
		},
	}

	h := httphandler.New(&options)
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/octet-stream"),
	)

	require.Len(t, errorLog, 2)
	require.EqualError(t, errorLog[0], `handler error`)
	require.EqualError(t, errorLog[1], `unable to encode "application/octet-stream": encoder error`)
}

func TestDoubleWrite(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		w.WriteHeader(http.StatusBadRequest)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "bad request")
		return &httphandler.HandlerError{
			StatusCode: http.StatusInternalServerError,
		}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusBadRequest),
		hit.Expect().Body().String().Equal("bad request"),
	)
}

func TestDefaultOptions(t *testing.T) {
	options := httphandler.Options{
		LogFunc:             nil,
		Encoders:            nil,
		FallbackEncoderFunc: nil,
		RequestUUIDFunc:     nil,
	}
	h := httphandler.New(&options)
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
		hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
		hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
	)
}

func TestSetEncoderOption(t *testing.T) {
	t.Run("using http handler", func(t *testing.T) {
		h := httphandler.New(nil)
		require.NoError(t, h.SetEncoder("application/json", func(w http.ResponseWriter, r *http.Request, e *httphandler.WireError) error {
			_, err := io.WriteString(w, "json")
			return err
		}))
		mux := http.NewServeMux()
		mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
			return &httphandler.HandlerError{
				ContentType: "application/json",
			}
		}))

		s := httptest.NewServer(mux)
		defer s.Close()

		hit.Test(t,
			hit.Get(s.URL),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Expect().Headers("Content-Type").Equal("application/json"),
			hit.Expect().Body().String().Equal("json"),
		)
	})
	t.Run("using options", func(t *testing.T) {
		var options httphandler.Options
		require.NoError(t, options.SetEncoder("application/json", func(w http.ResponseWriter, r *http.Request, e *httphandler.WireError) error {
			_, err := io.WriteString(w, "json")
			return err
		}))
		h := httphandler.New(&options)
		mux := http.NewServeMux()
		mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
			return &httphandler.HandlerError{
				ContentType: "application/json",
			}
		}))

		s := httptest.NewServer(mux)
		defer s.Close()

		hit.Test(t,
			hit.Get(s.URL),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Expect().Headers("Content-Type").Equal("application/json"),
			hit.Expect().Body().String().Equal("json"),
		)
	})
}

func TestSetEncodersOption(t *testing.T) {
	h := httphandler.New(nil)
	require.NoError(t, h.SetEncoders(map[string]httphandler.EncodeFunc{
		"application/json": func(w http.ResponseWriter, r *http.Request, e *httphandler.WireError) error {
			_, err := io.WriteString(w, "json")
			return err
		},
	}))
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{
			ContentType: "application/json",
		}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().String().Equal("json"),
	)
}

func TestSetLogFuncAndSetRequestUUIDFuncOption(t *testing.T) {
	h := httphandler.New(nil)
	require.NoError(t, h.SetLogFunc(func(handlerError error, internalError, publicError interface{}, statusCode int, requestUUID string) {
		require.EqualError(t, handlerError, "handler error")
		require.Nil(t, internalError)
		require.Equal(t, "unknown error", publicError)
		require.Equal(t, http.StatusInternalServerError, statusCode)
		require.Equal(t, "0123456789", requestUUID)
	}))
	require.NoError(t, h.SetRequestUUIDFunc(func() string {
		return "0123456789"
	}))
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{
			ContentType: "application/json",
		}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
		hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
		hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
	)
}

func TestSetFallbackEncoderOption(t *testing.T) {
	h := httphandler.New(nil)
	require.NoError(t, h.SetFallbackEncoder("application/json", func(w http.ResponseWriter, r *http.Request, e *httphandler.WireError) error {
		_, err := io.WriteString(w, "json")
		return err
	}))
	mux := http.NewServeMux()
	mux.HandleFunc("/", h.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().String().Equal("json"),
	)
}

func TestSetInvalidOptions(t *testing.T) {
	h := httphandler.New(nil)
	require.EqualError(t, h.SetLogFunc(nil), "logFunc cannot be nil")
	require.EqualError(t, h.SetEncoders(nil), "encoders cannot be nil")
	require.EqualError(t, h.SetEncoder("", func(_ http.ResponseWriter, _ *http.Request, _ *httphandler.WireError) error {
		return nil
	}), "content-type cannot be empty")
	require.EqualError(t, h.SetEncoder("text/html", nil), "encoder cannot be nil")
	require.EqualError(t, h.SetFallbackEncoder("", func(_ http.ResponseWriter, _ *http.Request, _ *httphandler.WireError) error {
		return nil
	}), "content-type cannot be empty")
	require.EqualError(t, h.SetFallbackEncoder("text/html", nil), "encoder cannot be nil")
	require.EqualError(t, h.SetRequestUUIDFunc(nil), "requestUUIDFunc cannot be nil")
}

func TestDefaultEncoders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{}
	}))

	s := httptest.NewServer(mux)
	defer s.Close()

	defaultEncoders := httphandler.DefaultOptions().Encoders

	for contentType := range defaultEncoders {
		hit.Test(t,
			hit.Get(s.URL),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Send().Headers("Accept").Add(contentType),
			hit.Expect().Headers("Content-Type").Equal(contentType),
		)
	}
}

func TestPanicHandler(t *testing.T) {
	t.Run("string", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
			panic("oops")
		}))
		s := httptest.NewServer(mux)
		defer s.Close()

		hit.Test(t,
			hit.Get(s.URL),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Expect().Headers("Content-Type").Equal("application/json"),
			hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
			hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
			hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
		)
	})

	t.Run("error", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
			panic(errors.New("oops"))
		}))
		s := httptest.NewServer(mux)
		defer s.Close()

		hit.Test(t,
			hit.Get(s.URL),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Expect().Headers("Content-Type").Equal("application/json"),
			hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
			hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
			hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
		)
	})

	t.Run("set custom panic handler", func(t *testing.T) {
		handler := httphandler.New(nil)
		handler.SetCustomPanicHandler(func(ctx context.Context, handlerError *httphandler.HandlerError) {
			require.Equal(t, "panic: oops", handlerError.InternalError)
		})
		mux := http.NewServeMux()
		mux.HandleFunc("/", handler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
			panic("oops")
		}))
		s := httptest.NewServer(mux)
		defer s.Close()

		hit.Test(t,
			hit.Get(s.URL),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Expect().Headers("Content-Type").Equal("application/json"),
			hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
			hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
			hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
		)
	})
}

func TestExtendedError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", httphandler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{
			PublicError: map[string]interface{}{
				"Title":   "Some Error",
				"Details": "Not implemented",
			},
		}
	}))
	s := httptest.NewServer(mux)
	defer s.Close()

	t.Run("application/json", func(t *testing.T) {
		hit.Test(t,
			hit.Get(s.URL),
			hit.Send().Headers("Accept").Add("application/json"),
			hit.Expect().Status().Equal(http.StatusInternalServerError),
			hit.Expect().Headers("Content-Type").Equal("application/json"),
			hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
			hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
			hit.Expect().Body().JSON().JQ(".Error.Title").Equal("Some Error"),
			hit.Expect().Body().JSON().JQ(".Error.Details").Equal("Not implemented"),
		)
	})
}

func TestRemoveEncoder(t *testing.T) {
	opts := httphandler.DefaultOptions()
	handler := httphandler.New(opts)
	// remove text/html encoders
	delete(opts.Encoders, "text/html")

	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.HandleFunc(func(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
		return &httphandler.HandlerError{
			PublicError: "unknown error",
		}
	}))
	s := httptest.NewServer(mux)
	defer s.Close()

	hit.Test(t,
		hit.Get(s.URL),
		// request text/html, however since we removed the encoders earlier we should fallback to application/json
		hit.Send().Headers("Accept").Add("text/html"),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
		hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
		hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
	)
}

type myHandler struct{}

func (h myHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) *httphandler.HandlerError {
	panic("implement me")
}

func TestHandler(t *testing.T) {
	s := httptest.NewServer(httphandler.Handle(myHandler{}))
	defer s.Close()
	hit.Test(t,
		hit.Get(s.URL),
		hit.Expect().Status().Equal(http.StatusInternalServerError),
		hit.Expect().Headers("Content-Type").Equal("application/json"),
		hit.Expect().Body().JSON().JQ(".StatusCode").Equal(http.StatusInternalServerError),
		hit.Expect().Body().JSON().JQ(".RequestUUID").Len().GreaterThan(0),
		hit.Expect().Body().JSON().JQ(".Error").Equal("unknown error"),
	)
}
