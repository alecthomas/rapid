package rapid

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/codegangsta/inject"
)

var (
	pathTransform = regexp.MustCompile(`{((\w+)(?::((?:\\.|[^}])+))?)}`)
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

// Protocol defining how responses and errors are encoded.
type Protocol interface {
	// TranslateError translates errors into
	TranslateError(in error) (status int, out error)
	WriteHeader(w http.ResponseWriter, r *http.Request, status int)
	EncodeResponse(w http.ResponseWriter, r *http.Request, status int, err error, data interface{})
	NotFound(w http.ResponseWriter, r *http.Request)
}

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
	route   *Route
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
	matches := make([]*routeMatch, 0, len(definition.routes))
	hr := reflect.ValueOf(handler)
	for _, route := range definition.routes {
		pattern, params := compilePath(route.path)
		method := hr.MethodByName(route.name)
		if !method.IsValid() {
			return nil, fmt.Errorf("no such method %s.%s", hr.Type(), route.name)
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
	i := inject.New()
	i.SetParent(s.Injector)
	i.MapTo(w, (*http.ResponseWriter)(nil))
	i.Map(r)
	match, parts := s.match(r)
	if match == nil {
		s.protocol.NotFound(w, r)
		return
	}
	var req interface{}
	if match.route.requestType != nil {
		req = reflect.New(match.route.requestType.Elem()).Interface()
		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			s.protocol.WriteHeader(w, r, http.StatusInternalServerError)
			s.protocol.EncodeResponse(w, r, http.StatusInternalServerError, err, nil)
			return
		}
		i.Map(req)
	}
	i.Map(parts)
	var closeNotifier CloseNotifierChannel
	if match.route.streamingResponse {
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
		if match.route.streamingResponse {
			panic("streaming responses must return (chan <type>, chan error)")
		}
		result = []reflect.Value{reflect.ValueOf((*struct{})(nil)), result[0]}

	case 2: // (response, error)
		// TODO: More checks for stuff.

	default:
		panic(fmt.Errorf("handler method %s.%s should return (<response>, <error>)", match.method.Type(), match.route.name))
	}
	if match.route.streamingResponse {
		s.log.Debug("%s %s -> streaming response", r.Method, r.URL)
		s.handleStream(closeNotifier, w, r, result[0], result[1])
	} else {
		s.log.Debug("%s %s -> %v", r.Method, r.URL, result[1].Interface())
		s.handleScalar(w, r, result[0], result[1])
	}
}

func (s *Server) handleScalar(w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerr reflect.Value) {
	status := http.StatusOK
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
		status, err = s.protocol.TranslateError(err)
		if err != nil {
			data = nil
		}
	}
	s.protocol.WriteHeader(w, r, status)
	s.protocol.EncodeResponse(w, r, status, err, data)

}

func (s *Server) writeResponse(w http.ResponseWriter, r *http.Request, status int, err error, data interface{}) {
	s.protocol.WriteHeader(w, r, status)
	s.protocol.EncodeResponse(w, r, status, err, data)
}

func (s *Server) handleStream(closeNotifier chan bool, w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerrs reflect.Value) {
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
		status, err := s.protocol.TranslateError(recv.Interface().(error))
		s.writeResponse(w, r, status, err, nil)
		return
	}

	// No error? Flush the 200 and star the main select loop.
	s.protocol.WriteHeader(w, r, http.StatusOK)
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
				status, err := s.protocol.TranslateError(recv.Interface().(error))
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
		if r.Method == match.route.httpMethod {
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

func compilePath(path string) (*regexp.Regexp, []string) {
	routePattern := "^" + path + "$"
	params := []string{}
	for _, match := range pathTransform.FindAllStringSubmatch(routePattern, -1) {
		pattern := `([^/]+)`
		if match[3] != "" {
			pattern = "(" + match[3] + ")"
			pattern = strings.Replace(pattern, `\{`, "{", -1)
			pattern = strings.Replace(pattern, `\}`, "}", -1)
		}
		routePattern = strings.Replace(routePattern, match[0], pattern, 1)
		params = append(params, match[2])
	}
	pattern := regexp.MustCompile(routePattern)
	return pattern, params
}
