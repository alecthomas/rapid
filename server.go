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
	"strings"

	"github.com/alecthomas/rapid/protocol"

	"github.com/codegangsta/inject"
)

var (
	pathTransform = regexp.MustCompile(`{((\w+)(\.\.\.)?)}`)
)

// Protocol defining how responses and errors are encoded.
type Protocol interface {
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

func Status(status int) error {
	return &HTTPStatus{status, http.StatusText(status)}
}

func StatusMessage(status int, message string) error {
	return &HTTPStatus{status, message}
}

func (h *HTTPStatus) Error() string {
	return h.Message
}

type Params map[string]string

type routeMatch struct {
	route   *Route
	pattern *regexp.Regexp
	params  []string
	method  reflect.Value
}

type Server struct {
	service  *Service
	matches  []*routeMatch
	protocol Protocol
}

func NewServer(service *Service, handler interface{}) (*Server, error) {
	matches := make([]*routeMatch, 0, len(service.Routes))
	hr := reflect.ValueOf(handler)
	for _, route := range service.Routes {
		pattern, params := compilePath(route.Path)
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
		service:  service,
		matches:  matches,
		protocol: &protocol.DefaultProtocol{},
	}
	return s, nil
}

func (s *Server) Protocol(protocol Protocol) *Server {
	s.protocol = protocol
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i := inject.New()
	i.MapTo(w, (*http.ResponseWriter)(nil))
	i.Map(r)
	match, parts := s.match(r)
	if match == nil {
		s.protocol.NotFound(w, r)
		return
	}
	var req interface{}
	if match.route.RequestType != nil {
		req = reflect.New(match.route.RequestType.Elem()).Interface()
		err := json.NewDecoder(r.Body).Decode(req)
		if err != nil {
			panic(err.Error())
		}
		i.Map(req)
	}
	i.Map(parts)
	result, err := i.Invoke(match.method.Interface())
	if err != nil {
		panic(err.Error())
	}
	if len(result) != 2 {
		panic(fmt.Errorf("handler method %s.%s should return (<response>, <error>)", match.method.Type(), match.route.Name))
	}
	if match.route.StreamingResponse {
		s.handleStream(w, r, result[0], result[1])
	} else {
		s.handleScalar(w, r, result[0], result[1])
	}
}

func (s *Server) handleScalar(w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerr reflect.Value) {
	status := http.StatusOK
	var data interface{}
	var err error
	if !rdata.IsNil() {
		data = rdata.Interface()
	}

	// If we have an error...
	if !rerr.IsNil() {
		err = rerr.Interface().(error)
		status, err = decodeError(err)
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

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request, rdata reflect.Value, rerrs reflect.Value) {
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

	s.protocol.WriteHeader(w, r, http.StatusOK)
	fw.Flush()

	cw := NewChunkedResponseWriter(w)
	defer cw.Close()

	rc := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: rdata}
	ec := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: rerrs}
	nc := reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(cn.CloseNotify())}
	cases := []reflect.SelectCase{rc, ec, nc}
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
				status, err := decodeError(recv.Interface().(error))
				s.protocol.EncodeResponse(cw, r, status, err, nil)
				return

			case 2: // CloseNotifier
				return
			}
		} else {
			return
		}
	}
}

func decodeError(err error) (int, error) {
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

func (s *Server) match(r *http.Request) (*routeMatch, Params) {
	for _, match := range s.matches {
		if r.Method == match.route.HTTPMethod {
			matches := match.pattern.FindStringSubmatch(r.URL.Path)
			if matches != nil {
				params := Params{}
				for i, k := range match.params {
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
	for _, match := range pathTransform.FindAllStringSubmatch(routePattern, 16) {
		pattern := `([^/]+)`
		if match[3] == "..." {
			pattern = `(.+)`
		}
		routePattern = strings.Replace(routePattern, match[0], pattern, 1)
	}
	pattern := regexp.MustCompile(routePattern)
	params := []string{}
	for _, arg := range pathTransform.FindAllString(path, 16) {
		params = append(params, arg[1:len(arg)-1])
	}
	return pattern, params
}
