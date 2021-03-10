package httphandler

import (
	"context"
	"mime"
	"net/http"
	"strings"

	"github.com/pkg/errors"
)

// Handler provides a HandleFunc function that can be used to return errors based on the client "Accept" header value.
type Handler struct {
	options *Options
}

// New constructs a new Handler with the specified Options.
// To construct with default options use New(nil) or use the DefaultHandler.
func New(options *Options) *Handler {
	if options == nil {
		return &Handler{options: defaultOptions()}
	}
	if options.LogFunc == nil {
		options.LogFunc = defaultLogFunc()
	}
	if options.Encoders == nil {
		options.Encoders = defaultEncoders()
	} else {
		_ = options.SetEncoders(options.Encoders)
	}
	if options.FallbackEncoderFunc == nil {
		options.FallbackEncoderFunc = defaultFallbackEncoder()
	}
	if options.RequestUUIDFunc == nil {
		options.RequestUUIDFunc = defaultRequestUUID()
	}
	if options.CustomPanicHandler == nil {
		options.CustomPanicHandler = defaultCustomPanicHandler()
	}
	return &Handler{options: options}
}

// HandlerError represents the error that should be returned from the handler func in case of error.
type HandlerError struct {
	// StatusCode is the http status code to send to the client.
	// If not specified HandleFunc will use http.StatusInternalServerError.
	StatusCode int
	// PublicError is the error that will be visible to the client. Do not include sensitive information here.
	PublicError error
	// InternalError is the error that will not be visible to the client.
	InternalError error
	// ContentType specifies the Content-Type of this error. If not specified HandleFunc will use the clients Accept
	// header. If specified the clients Accept header will be ignored.
	ContentType string
}

// WireError represents the error that will be send "over the wire" to the client.
type WireError struct {
	// StatusCode is the http status code that was sent to the client.
	StatusCode int
	// Error is the error message that should be send to the client.
	Error string
	// RequestUUID is the request uuid that should be send to the client.
	RequestUUID string
}

// PanicHandler is the type for custom functions for handling panics.
type PanicHandler func(context.Context, *HandlerError)

// The HandlerFunc type is an adapter to allow the use of
// ordinary functions as HTTP handlers. If f is a function
// with the appropriate signature, HandlerFunc(f) is a
// Handler that calls f.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) *HandlerError

// HandleFunc wraps a handler with a HandlerError return value.
// In case the provided handler function returns an error, HandleFunc will construct a response based on the error and
// the Accept header of the client.
// If the HandlerError specifies a ContentType value the clients Accept header will be ignored.
// If the provided handler function returns no error no action will be taken, this means that the specified handler func
// is required to send the http headers, status code and body.
//
// Example:
//     http.HandleFunc(HandleFunc(func(w http.ResponseWriter, r *http.Request) *HandlerError {
//         return &HandlerError{
//             StatusCode: http.StatusUnauthorized,
//             PublicError: errors.New("you have no permission to view this site"),
//             InternalError: errors.New("client authentication failed"),
//         }
//     })
func (h *Handler) HandleFunc(handler func(w http.ResponseWriter, r *http.Request) *HandlerError) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		safeWriter := newSafeResponseWriter(w)
		requestUUID := h.options.RequestUUIDFunc()
		requestWithContext := r.WithContext(context.WithValue(r.Context(), uuidKey, requestUUID))

		err := safeHandlerCall(handler, safeWriter, requestWithContext, h.options.CustomPanicHandler)
		if err == nil {
			return
		}

		if err.StatusCode == 0 {
			err.StatusCode = http.StatusInternalServerError
		}
		if err.PublicError == nil {
			err.PublicError = errors.New("unknown error")
		}
		h.options.LogFunc(
			errors.New("handler error"),
			err.InternalError,
			err.PublicError,
			err.StatusCode,
			requestUUID,
		)

		// we have written already
		if safeWriter.Written() {
			return
		}

		h.sendError(err, requestUUID, safeWriter, requestWithContext)
	}
}

func (h *Handler) sendError(err *HandlerError, requestUUID string, w http.ResponseWriter, r *http.Request) {
	errorToSend := &WireError{
		StatusCode:  err.StatusCode,
		Error:       err.PublicError.Error(),
		RequestUUID: requestUUID,
	}

	var f EncodeFunc

	if err.ContentType == "" {
		f, err.ContentType = getPreferredContentType(h.options, r)
	} else {
		err.ContentType = strings.ToLower(err.ContentType)
		f = h.options.Encoders[err.ContentType]
	}

	if f == nil || err.ContentType == "" {
		// use fallback
		f, err.ContentType = h.options.FallbackEncoderFunc()
		err.ContentType = strings.ToLower(err.ContentType)
	}

	w.Header().Set("Content-Type", err.ContentType)
	w.WriteHeader(err.StatusCode)
	if encodeErr := f(w, r, errorToSend); encodeErr != nil {
		h.options.LogFunc(
			errors.Wrapf(encodeErr, "unable to encode %q", err.ContentType),
			err.InternalError,
			err.PublicError,
			err.StatusCode,
			requestUUID,
		)
	}
}

// SetLogFunc sets the log function that will be called in case of error.
func (h *Handler) SetLogFunc(logFunc LogFunc) error {
	return h.options.SetLogFunc(logFunc)
}

// SetEncoders sets the Encoders to the specified map of content type and EncodeFunc.
// It will be used to lookup the encoder for the error content type.
func (h *Handler) SetEncoders(encoders map[string]EncodeFunc) error {
	return h.options.SetEncoders(encoders)
}

// SetEncoder sets one specific encoder in the Encoders map.
func (h *Handler) SetEncoder(contentType string, encoder EncodeFunc) error {
	return h.options.SetEncoder(contentType, encoder)
}

// SetFallbackEncoder sets the fallback encoder in case the error Content-Type does not exist in the
// Encoders map.
func (h *Handler) SetFallbackEncoder(contentType string, encoder EncodeFunc) error {
	return h.options.SetFallbackEncoder(contentType, encoder)
}

// SetRequestUUIDFunc specifies the function that returns an request uuid. This request uuid will be send to the
// LogFunc in case of error.
// The request uuid is also available in the specified handler (in HandleFunc()) by using GetRequestUUID().
func (h *Handler) SetRequestUUIDFunc(requestUUIDFunc func() string) error {
	return h.options.SetRequestUUIDFunc(requestUUIDFunc)
}

func safeHandlerCall(h HandlerFunc, w http.ResponseWriter, r *http.Request, ph PanicHandler) (err *HandlerError) {
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		switch v := e.(type) {
		case error:
			err = &HandlerError{
				InternalError: errors.Wrap(v, "panic"),
			}
		default:
			err = &HandlerError{
				InternalError: errors.Errorf("panic: %v", v),
			}
		}
		ph(r.Context(), err)
	}()
	err = h(w, r)
	return err
}

func getPreferredContentType(options *Options, r *http.Request) (enocder EncodeFunc, contentType string) {
	// if the request has a Accept header use this header to determinate the output format
	if accept := r.Header.Values("Accept"); len(accept) != 0 {
		for _, s := range accept {
			mediaType, _, err := mime.ParseMediaType(s)
			if err != nil {
				continue
			}
			ct := strings.ToLower(mediaType)
			f, ok := options.Encoders[ct]
			if ok {
				return f, ct
			}
		}
	}
	return nil, ""
}

// DefaultHandler is the default instance that can be used out of the box.
// It uses the default settings.
var DefaultHandler = New(nil)

// HandleFunc wraps a handler with a HandlerError return value.
// In case the provided handler function returns an error, HandleFunc will construct a response based on the error and
// the Accept header of the client.
// If the HandlerError specifies a ContentType value the clients Accept header will be ignored.
// If the provided handler function returns no error no action will be taken, this means that the specified handler func
// is required to send the http headers, status code and body.
//
// Example:
//     http.HandleFunc(HandleFunc(func(w http.ResponseWriter, r *http.Request) *HandlerError {
//         return &HandlerError{
//             StatusCode: http.StatusUnauthorized,
//             PublicError: errors.New("you have no permission to view this site"),
//             InternalError: errors.New("client authentication failed"),
//         }
//     })
func HandleFunc(handler func(w http.ResponseWriter, r *http.Request) *HandlerError) func(w http.ResponseWriter, r *http.Request) {
	return DefaultHandler.HandleFunc(handler)
}
