package httphandler

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/google/uuid"
)

// LogFunc is the log function that will be called in case of error.
type LogFunc func(handlerError, internalError, publicError error, statusCode int, requestUUID string)

// EncodeFunc is the encode function that will be called to encode the WireError in the desired format.
type EncodeFunc func(http.ResponseWriter, *http.Request, *WireError) error

// Options is a structure that should be passed into New() it defines and controls behavior of HandleFunc().
type Options struct {
	// LogFunc is the log function that will be called in case of error.
	// If LogFunc is nil the default logger will be used.
	LogFunc LogFunc
	// Encoders is a map of Content-Type and EncodeFunc, it will be used to lookup the encoder for the Content-Type.
	// If Encoder is nil the default encoders will be used.
	Encoders map[string]EncodeFunc
	// FallbackEncoderFunc should return a fallback encoder in case the error Content-Type does not exist in the
	// Encoders map.
	// If FallbackEncoderFunc is nil the default fallback encoder will be used.
	FallbackEncoderFunc func() (EncodeFunc, string)
	// RequestUUIDFunc specifies the function that returns an request uuid. This request uuid will be send to the
	// LogFunc in case of error.
	// The RequestUUID is also available in the specified handler (in HandleFunc()) by using GetRequestUUID().
	// If RequestUUIDFunc is nil the default request uuid func will be used.
	RequestUUIDFunc func() string
	// CustomPanicHandler it's called when a panic occurrs in the HTTP handler. It gets the request context value.
	CustomPanicHandler PanicHandler
}

// SetLogFunc sets the log function that will be called in case of error.
func (o *Options) SetLogFunc(logFunc LogFunc) error {
	if logFunc == nil {
		return errors.New("logFunc cannot be nil")
	}
	o.LogFunc = logFunc
	return nil
}

// SetEncoders sets the Encoders to the specified map of content type and EncodeFunc.
// It will be used to lookup the encoder for the error content type.
func (o *Options) SetEncoders(encoders map[string]EncodeFunc) error {
	if encoders == nil {
		return errors.New("encoders cannot be nil")
	}
	for contentType, encoder := range encoders {
		o.Encoders[strings.ToLower(contentType)] = encoder
	}
	return nil
}

// SetEncoder sets one specific encoder in the Encoders map.
func (o *Options) SetEncoder(contentType string, encoder EncodeFunc) error {
	if contentType == "" {
		return errors.New("content-type cannot be empty")
	}
	if encoder == nil {
		return errors.New("encoder cannot be nil")
	}
	if o.Encoders == nil {
		o.Encoders = make(map[string]EncodeFunc)
	}
	o.Encoders[strings.ToLower(contentType)] = encoder
	return nil
}

// SetFallbackEncoder sets the fallback encoder in case the error Content-Type does not exist in the
// Encoders map.
func (o *Options) SetFallbackEncoder(contentType string, encoder EncodeFunc) error {
	if contentType == "" {
		return errors.New("content-type cannot be empty")
	}
	if encoder == nil {
		return errors.New("encoder cannot be nil")
	}
	o.FallbackEncoderFunc = func() (EncodeFunc, string) {
		return encoder, contentType
	}
	return nil
}

// SetRequestUUIDFunc specifies the function that returns an request uuid. This request uuid will be send to the
// LogFunc in case of error.
// The request uuid is also available in the specified handler (in HandleFunc()) by using GetRequestUUID().
func (o *Options) SetRequestUUIDFunc(requestUUIDFunc func() string) error {
	if requestUUIDFunc == nil {
		return errors.New("requestUUIDFunc cannot be nil")
	}
	o.RequestUUIDFunc = requestUUIDFunc
	return nil
}

func (o *Options) SetCustomPanicHandler(f func(context.Context, *HandlerError)) {
	o.CustomPanicHandler = f
}

func defaultOptions() *Options {
	return &Options{
		LogFunc:             defaultLogFunc(),
		Encoders:            defaultEncoders(),
		FallbackEncoderFunc: defaultFallbackEncoder(),
		RequestUUIDFunc:     defaultRequestUUID(),
		CustomPanicHandler:  defaultCustomPanicHandler(),
	}
}

func defaultLogFunc() LogFunc {
	return func(handlerError, internalError, publicError error, statusCode int, requestUUID string) {
		log.Printf("%v: internalError=%v, publicError=%v, statusCode=%d, requestUUID=%s",
			handlerError,
			internalError,
			publicError,
			statusCode,
			requestUUID,
		)
	}
}

func defaultEncoders() map[string]EncodeFunc {
	return map[string]EncodeFunc{
		"application/json": func(w http.ResponseWriter, r *http.Request, e *WireError) error {
			return json.NewEncoder(w).Encode(e)
		},
		"application/xml": func(w http.ResponseWriter, r *http.Request, e *WireError) error {
			return xml.NewEncoder(w).Encode(e)
		},
		"text/html": func(w http.ResponseWriter, r *http.Request, e *WireError) error {
			if _, err := io.WriteString(w, "<!DOCTYPE html><html><head><title>"); err != nil {
				return err
			}
			if _, err := io.WriteString(w, strconv.Itoa(e.StatusCode)); err != nil {
				return err
			}
			if _, err := io.WriteString(w, " Error</title></head><body><h1>"); err != nil {
				return err
			}
			if _, err := io.WriteString(w, e.Error); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "<hr>"); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "<p>RequestUUID: <code>"); err != nil {
				return err
			}
			if _, err := io.WriteString(w, e.RequestUUID); err != nil {
				return err
			}
			if _, err := io.WriteString(w, "</code></p></body></html>"); err != nil {
				return err
			}
			return nil
		},
		"text/xml": func(w http.ResponseWriter, r *http.Request, e *WireError) error {
			return xml.NewEncoder(w).Encode(e)
		},
	}
}

func defaultFallbackEncoder() func() (EncodeFunc, string) {
	return func() (EncodeFunc, string) {
		return func(w http.ResponseWriter, r *http.Request, e *WireError) error {
			return json.NewEncoder(w).Encode(e)
		}, "application/json"
	}
}

func defaultRequestUUID() func() string {
	return func() string {
		return uuid.New().String()
	}
}

func defaultCustomPanicHandler() PanicHandler {
	return func(ctx context.Context, err *HandlerError) {}
}
