package rapid

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"time"

	"github.com/codegangsta/inject"
	structschema "github.com/gorilla/schema"
)

var schemadecoder *structschema.Decoder

func init() {
	schemadecoder = structschema.NewDecoder()
	schemadecoder.RegisterConverter(time.Duration(0), convertDuration)
	schemadecoder.RegisterConverter(time.Time{}, convertTime)
}

func convertDuration(value string) reflect.Value {
	if d, err := time.ParseDuration(value); err == nil {
		return reflect.ValueOf(d)
	}
	return reflect.Value{}
}

func convertTime(value string) reflect.Value {
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return reflect.ValueOf(t)
	}
	return reflect.Value{}
}

type Validator interface {
	Validate() error
}

type Logger interface {
	Debugf(fmt string, args ...interface{})
	Infof(fmt string, args ...interface{})
	Warningf(fmt string, args ...interface{})
	Errorf(fmt string, args ...interface{})
}

type loggerSink struct{}

func (l *loggerSink) Debugf(fmt string, args ...interface{})   {}
func (l *loggerSink) Infof(fmt string, args ...interface{})    {}
func (l *loggerSink) Warningf(fmt string, args ...interface{}) {}
func (l *loggerSink) Errorf(fmt string, args ...interface{})   {}

type CloseNotifierChannel <-chan bool

// An error-conformant type that can return a HTTP status code, a message, and
// optional headers.
type HTTPStatus struct {
	Status  int         `json:"status"`
	Message string      `json:"error"`
	Headers http.Header `json:"-"`
}

func (h *HTTPStatus) Error() string {
	return h.Message
}

func ErrorForStatus(status int) error {
	return Error(status, http.StatusText(status))
}

func Error(status int, message string) error {
	return ErrorWithHeaders(status, message, http.Header{})
}

func ErrorForStatusWithHeaders(status int, headers http.Header) error {
	return &HTTPStatus{status, http.StatusText(status), headers}
}

func ErrorWithHeaders(status int, message string, headers http.Header) error {
	return &HTTPStatus{status, message, headers}
}

type Params map[string]string

func (p Params) Int(key string) (int64, error) {
	v, ok := p[key]
	if !ok {
		return 0, fmt.Errorf("no such query parameter %s", key)
	}
	return strconv.ParseInt(v, 10, 64)
}

func (p Params) Float(key string) (float64, error) {
	v, ok := p[key]
	if !ok {
		return 0, fmt.Errorf("no such query parameter %s", key)
	}
	return strconv.ParseFloat(v, 64)
}

type routeMatch struct {
	route   *RouteSchema
	pattern *regexp.Regexp
	params  []string
	method  reflect.Value
}

// A function with the signature f(...) error. Arguments can be injected.
type BeforeHandlerFunc interface{}

type Server struct {
	schema        *Schema
	matches       []*routeMatch
	codec         CodecFactory
	log           Logger
	Injector      inject.Injector
	handler       interface{}
	beforeHandler BeforeHandlerFunc
}

func NewServer(schema *Schema, handler interface{}) (*Server, error) {
	matches := []*routeMatch{}
	hr := reflect.ValueOf(handler)
	for _, resource := range schema.Resources {
		for _, route := range resource.Routes {
			pattern, params := route.CompilePath()
			method := hr.MethodByName(route.Name)
			if !method.IsValid() {
				return nil, fmt.Errorf("no such method %s.%s", hr.Type(), route.Name)
			}
			matches = append(matches, &routeMatch{
				route:   route,
				pattern: pattern,
				params:  params,
				method:  method,
			})
		}
	}
	s := &Server{
		schema:   schema,
		matches:  matches,
		codec:    DefaultCodecFactory,
		log:      &loggerSink{},
		Injector: inject.New(),
		handler:  handler,
	}
	return s, nil
}

// Specify the default CodecFactory for the server.
func (s *Server) Codec(codec CodecFactory) *Server {
	s.codec = codec
	return s
}

func (s *Server) Logger(log Logger) *Server {
	s.log = log
	return s
}

func (s *Server) BeforeHandler(before BeforeHandlerFunc) *Server {
	s.beforeHandler = before
	return s
}

// Close underlying handler if it supports io.Closer.
func (s *Server) Close() error {
	if closer, ok := s.handler.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

func indirect(t reflect.Type) reflect.Type {
	if t.Kind() == reflect.Ptr {
		return indirect(t.Elem())
	}
	return t
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.log.Debugf("%s %s", r.Method, r.URL)

	// Match URL and method.
	match, parts := s.match(r)
	if match == nil {
		s.codec.Response(nil).EncodeResponse(r, w, http.StatusNotFound, nil)
		return
	}

	i := inject.New()
	i.SetParent(s.Injector)

	// Decode path parameters, if any.
	if match.route.PathType != nil {
		path := reflect.New(indirect(match.route.PathType)).Interface()
		values := url.Values{}
		for key, value := range parts {
			values.Add(key, value)
		}
		err := schemadecoder.Decode(path, values)
		if err != nil {
			s.codec.Response(nil).EncodeResponse(r, w, http.StatusBadRequest, err)
			return
		}
		if v, ok := path.(Validator); ok {
			if err := v.Validate(); err != nil {
				s.codec.Response(nil).EncodeResponse(r, w, http.StatusBadRequest, err)
				return
			}
		}
		i.Map(path)
	}

	// Decode query parameters, if any.
	if match.route.QueryType != nil {
		query := reflect.New(indirect(match.route.QueryType)).Interface()
		err := schemadecoder.Decode(query, r.URL.Query())
		if err != nil {
			s.codec.Response(nil).EncodeResponse(r, w, http.StatusBadRequest, err)
			return
		}
		if v, ok := query.(Validator); ok {
			if err := v.Validate(); err != nil {
				s.codec.Response(nil).EncodeResponse(r, w, http.StatusBadRequest, err)
				return
			}
		}
		i.Map(query)
	}

	// Decode request body, if any.
	if match.route.RequestType != nil {
		req := reflect.New(indirect(match.route.RequestType)).Interface()
		err := s.codec.Request(req).DecodeRequest(r)
		if err != nil {
			s.codec.Response(nil).EncodeResponse(r, w, http.StatusBadRequest, err)
			return
		}
		if v, ok := req.(Validator); ok {
			if err := v.Validate(); err != nil {
				s.codec.Response(nil).EncodeResponse(r, w, http.StatusBadRequest, err)
				return
			}
		}
		i.Map(req)
	}

	i.MapTo(i, (*inject.Injector)(nil))
	i.MapTo(w, (*http.ResponseWriter)(nil))
	i.Map(r)
	i.Map(parts)
	i.Map(match.route)

	var closeNotifier CloseNotifierChannel
	if cn, ok := w.(http.CloseNotifier); ok {
		// WriteError(w, http.StatusInternalServerError, errors.New("HTTP writer does not support close notifications"))
		// return
		closeNotifier = CloseNotifierChannel(cn.CloseNotify())
		i.Map(closeNotifier)
	}

	if s.beforeHandler != nil {
		results, err := i.Invoke(s.beforeHandler)
		if err != nil {
			panic(err.Error())
		}
		rerr := results[0]
		if !rerr.IsNil() {
			err = rerr.Interface().(error)
			if err != nil {
				s.codec.Response(nil).EncodeResponse(r, w, 500, err)
				return
			}
		}
	}

	result, err := i.Invoke(match.method.Interface())
	if err != nil {
		panic(err.Error())
	}
	switch len(result) {
	case 0: // Zero return values, we assume the handler has processed the request itself.
		return

	case 1: // Single value is always an error, so we just synthesize (nil, error).
		result = []reflect.Value{reflect.ValueOf((*struct{})(nil)), result[0]}

	case 2: // (response, error)
		// TODO: More checks for stuff.

	default:
		panic(fmt.Errorf("handler method %s.%s should return (<response>, <error>)", match.method.Type(), match.route.Name))
	}
	s.log.Debugf("%s %s -> %v", r.Method, r.URL, result[1].Interface())
	s.handleScalar(match.route, closeNotifier, w, r, result[0], result[1])
}

func (s *Server) handleScalar(route *RouteSchema, closeNotifier CloseNotifierChannel, w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerr reflect.Value) {
	var data interface{}
	var err error
	switch rdata.Kind() {
	case reflect.String:
		data = rdata.String()

	case reflect.Int, reflect.Int16, reflect.Int32, reflect.Int64:
		data = rdata.Int()

	case reflect.Uint, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		data = rdata.Uint()

	case reflect.Float32, reflect.Float64:
		data = rdata.Float()

	default:
		if !rdata.IsNil() {
			data = rdata.Interface()
		}

	}

	// If we have an error...
	if !rerr.IsNil() {
		err = rerr.Interface().(error)
	}
	s.codec.Response(data).EncodeResponse(r, w, 0, err)
}

func (s *Server) match(r *http.Request) (*routeMatch, Params) {
	for _, match := range s.matches {
		if r.Method == match.route.Method {
			matches := match.pattern.FindStringSubmatch(r.URL.Path)
			if matches != nil {
				params := Params{}
				for i, k := range match.params {
					// fmt.Printf("%s = %s (%d)\n", k, matches[i+1], i+1)
					params[k] = matches[i+1]
				}
				return match, params
			}
		}
	}
	return nil, nil
}
