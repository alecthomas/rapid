package rapid

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	EncodeResponse(w http.ResponseWriter, r *http.Request, code int, err error, payload interface{})
	NotFound(w http.ResponseWriter, r *http.Request)
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
	w.Header().Set("Content-Type", "application/json")

	if match.route.StreamingResponse {
		w.WriteHeader(http.StatusOK)
		ec := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: result[2],
		}
		rc := reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: result[1],
		}
		cases := []reflect.SelectCase{rc, ec}
		for {
			if chosen, recv, ok := reflect.Select(cases); ok {
				switch chosen {
				case 0: // value
					println(recv.String())

				case 1: // error
				}
			} else {
				return
			}
		}
	} else {
		status := http.StatusOK
		var data interface{}
		var err error
		if !result[0].IsNil() {
			data = result[0].Interface()
		}

		// If we have an error...
		if !result[1].IsNil() {
			err = result[1].Interface().(error)
			// Check if it's a HTTPStatus error, in which case check the status code.
			if st, ok := err.(*HTTPStatus); ok {
				status = st.Status
				// If it's not an error, clear the error value so we don't return an error value.
				if st.Status >= 200 || st.Status <= 299 {
					err = nil
				} else {
					// If it *is* an error, clear the data so we don't return data.
					data = nil
				}
			} else {
				// If it's any other error type, set 500 and continue.
				status = http.StatusInternalServerError
				err = errors.New("internal server error")
			}
		}
		s.protocol.EncodeResponse(w, r, status, err, data)
	}
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
