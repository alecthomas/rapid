package rapid

import (
	"net/http"
	"reflect"

	"github.com/codegangsta/inject"
)

type Converter interface {
	Convert(value string, injector *inject.Injector)
}

type Service struct {
	Name   string
	Routes []*Route
}

func NewService(name string) *Service {
	return &Service{Name: name}
}

func (s *Service) Route(name string) *Route {
	route := newRoute(name)
	s.Routes = append(s.Routes, route)
	return route
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

type Route struct {
	Name              string
	Path              string
	HTTPMethod        string
	StreamingResponse bool
	RequestType       reflect.Type
	ResponseType      reflect.Type
}

func newRoute(name string) *Route {
	return &Route{
		Name: name,
	}
}

func (r *Route) Method(method, path string) *Route {
	r.HTTPMethod = method
	r.Path = path
	return r
}

func (r *Route) Any(path string) *Route {
	return r.Method("", path)
}

func (r *Route) Post(path string) *Route {
	return r.Method("POST", path)
}

func (r *Route) Get(path string) *Route {
	return r.Method("GET", path)
}

func (r *Route) Put(path string) *Route {
	return r.Method("PUT", path)
}

func (r *Route) Delete(path string) *Route {
	return r.Method("DELETE", path)
}

func (r *Route) Options(path string) *Route {
	return r.Method("OPTIONS", path)
}

func (r *Route) Request(req interface{}) *Route {
	r.RequestType = reflect.TypeOf(req)
	return r
}

func (r *Route) Response(resp interface{}) *Route {
	r.ResponseType = reflect.TypeOf(resp)
	return r
}

func (r *Route) Streaming() *Route {
	r.StreamingResponse = true
	return r
}
