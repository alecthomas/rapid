package rapid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"reflect"
	"regexp"
	"strconv"

	"github.com/alecthomas/rapid/schema"
	"github.com/codegangsta/inject"
	structschema "github.com/gorilla/schema"
)

type Logger interface {
	Debug(fmt string, args ...interface{})
	Info(fmt string, args ...interface{})
	Warning(fmt string, args ...interface{})
	Error(fmt string, args ...interface{})
}

type loggerSink struct{}

func (l *loggerSink) Debug(fmt string, args ...interface{})   {}
func (l *loggerSink) Info(fmt string, args ...interface{})    {}
func (l *loggerSink) Warning(fmt string, args ...interface{}) {}
func (l *loggerSink) Error(fmt string, args ...interface{})   {}

type CloseNotifierChannel chan bool

type ChunkedResponseWriter struct {
	w  http.ResponseWriter
	cw io.WriteCloser
}

func NewChunkedResponseWriter(w http.ResponseWriter) *ChunkedResponseWriter {
	return &ChunkedResponseWriter{
		w:  w,
		cw: httputil.NewChunkedWriter(w),
	}
}

func (c *ChunkedResponseWriter) Header() http.Header {
	return c.w.Header()
}

func (c *ChunkedResponseWriter) Write(b []byte) (int, error) {
	return c.cw.Write(b)
}
func (c *ChunkedResponseWriter) WriteHeader(status int) {
	c.WriteHeader(status)
}

func (c *ChunkedResponseWriter) Close() error {
	return c.cw.Close()
}

type HTTPStatus struct {
	Status  int
	Message string
}

func ErrorForStatus(status int) error {
	return &HTTPStatus{status, http.StatusText(status)}
}

func Error(status int, message string) error {
	return &HTTPStatus{status, message}
}

func (h *HTTPStatus) Error() string {
	return h.Message
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
	route   *schema.Route
	pattern *regexp.Regexp
	params  []string
	method  reflect.Value
}

type Server struct {
	definition *Definition
	matches    []*routeMatch
	protocol   Protocol
	log        Logger
	Injector   inject.Injector
}

func NewServer(definition *Definition, handler interface{}) (*Server, error) {
	matches := make([]*routeMatch, 0, len(definition.Schema.Routes))
	hr := reflect.ValueOf(handler)
	for _, route := range definition.Schema.Routes {
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
	s := &Server{
		definition: definition,
		matches:    matches,
		protocol:   &DefaultProtocol{},
		log:        &loggerSink{},
		Injector:   inject.New(),
	}
	s.Injector.MapTo(s.protocol, (*Protocol)(nil))
	return s, nil
}

func (s *Server) SetProtocol(protocol Protocol) *Server {
	s.protocol = protocol
	return s
}

func (s *Server) SetLogger(log Logger) *Server {
	s.log = log
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.log.Debug("%s %s", r.Method, r.URL)

	// Match URL and method.
	match, parts := s.match(r)
	if match == nil {
		s.protocol.NotFound(w, r)
		return
	}

	i := inject.New()
	i.SetParent(s.Injector)

	// Decode path parameters, if any.
	var path interface{}
	if match.route.PathType != nil {
		path = reflect.New(match.route.PathType.Elem()).Interface()
		values := url.Values{}
		for key, value := range parts {
			values.Add(key, value)
		}
		err := structschema.NewDecoder().Decode(path, values)
		if err != nil {
			s.protocol.WriteHeader(w, r, http.StatusBadRequest)
			s.protocol.EncodeResponse(w, r, http.StatusBadRequest, err, nil)
			return
		}
		i.Map(path)
	}

	// Decode query parameters, if any.
	var query interface{}
	if match.route.QueryType != nil {
		query = reflect.New(match.route.QueryType.Elem()).Interface()
		err := structschema.NewDecoder().Decode(query, r.URL.Query())
		if err != nil {
			s.protocol.WriteHeader(w, r, http.StatusBadRequest)
			s.protocol.EncodeResponse(w, r, http.StatusBadRequest, err, nil)
			return
		}
		i.Map(query)
	}

	// Decode rqeuest body, if any.
	var req interface{}
	if match.route.RequestType != nil {
		req = reflect.New(match.route.RequestType.Elem()).Interface()
		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			s.protocol.WriteHeader(w, r, http.StatusBadRequest)
			s.protocol.EncodeResponse(w, r, http.StatusBadRequest, err, nil)
			return
		}
		i.Map(req)
	}

	i.MapTo(w, (*http.ResponseWriter)(nil))
	i.Map(r)
	i.Map(parts)

	var closeNotifier CloseNotifierChannel
	if match.route.StreamingResponse {
		closeNotifier = make(CloseNotifierChannel)
		i.Map(closeNotifier)
	}
	result, err := i.Invoke(match.method.Interface())
	if err != nil {
		panic(err.Error())
	}
	switch len(result) {
	case 0: // Zero return values, we assume the handler has processed the request itself.
		return

	case 1: // Single value is always an error, so we just synthesize (nil, error).
		if match.route.StreamingResponse {
			panic("streaming responses must return (chan <type>, chan error)")
		}
		result = []reflect.Value{reflect.ValueOf((*struct{})(nil)), result[0]}

	case 2: // (response, error)
		// TODO: More checks for stuff.

	default:
		panic(fmt.Errorf("handler method %s.%s should return (<response>, <error>)", match.method.Type(), match.route.Name))
	}
	if match.route.StreamingResponse {
		s.log.Debug("%s %s -> streaming response", r.Method, r.URL)
		s.handleStream(match.route, closeNotifier, w, r, result[0], result[1])
	} else {
		s.log.Debug("%s %s -> %v", r.Method, r.URL, result[1].Interface())
		s.handleScalar(match.route, w, r, result[0], result[1])
	}
}

func (s *Server) handleScalar(route *schema.Route, w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerr reflect.Value) {
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
	status, err := s.protocol.TranslateError(r, route.SuccessStatus, err)
	if err != nil {
		data = nil
	}
	s.protocol.WriteHeader(w, r, status)
	s.protocol.EncodeResponse(w, r, status, err, data)

}

func (s *Server) writeResponse(w http.ResponseWriter, r *http.Request, status int, err error, data interface{}) {
	s.protocol.WriteHeader(w, r, status)
	s.protocol.EncodeResponse(w, r, status, err, data)
}

func (s *Server) handleStream(route *schema.Route, closeNotifier chan bool, w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerrs reflect.Value) {
	fw, ok := w.(http.Flusher)
	if !ok {
		s.writeResponse(w, r, http.StatusInternalServerError, errors.New("HTTP writer does not support flushing"), nil)
		return
	}

	cn, ok := w.(http.CloseNotifier)
	if !ok {
		s.writeResponse(w, r, http.StatusInternalServerError, errors.New("HTTP writer does not support close notifications"), nil)
		return
	}

	cw := NewChunkedResponseWriter(w)

	// If we have an immediate error, return this directly in the HTTP
	// response rather than streaming it.
	ec := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: rerrs}
	dc := reflect.SelectCase{Dir: reflect.SelectDefault}
	cases := []reflect.SelectCase{dc, ec}
	if _, recv, ok := reflect.Select(cases); ok {
		status, err := s.protocol.TranslateError(r, 0, recv.Interface().(error))
		s.writeResponse(w, r, status, err, nil)
		return
	}

	status, _ := s.protocol.TranslateError(r, route.SuccessStatus, nil)
	// No error? Flush the 200 and star the main select loop.
	s.protocol.WriteHeader(w, r, status)
	fw.Flush()
	defer cw.Close()

	rc := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: rdata}
	nc := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(cn.CloseNotify())}
	cases = []reflect.SelectCase{rc, ec, nc}
	for {
		if chosen, recv, ok := reflect.Select(cases); ok {
			switch chosen {
			case 0: // value
				data := recv.Interface()
				if data == nil {
					return
				}
				s.protocol.EncodeResponse(cw, r, http.StatusOK, nil, data)
				fw.Flush()

			case 1: // error
				status, err := s.protocol.TranslateError(r, 0, recv.Interface().(error))
				s.protocol.EncodeResponse(cw, r, status, err, nil)
				return

			case 2: // CloseNotifier
				s.log.Debug("HTTP connection closed")
				closeNotifier <- true
				return
			}
		}
	}
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
