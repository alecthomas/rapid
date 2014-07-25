package rapid

import (
	"reflect"

	"github.com/codegangsta/inject"
)

type Converter interface {
	Convert(value string, injector *inject.Injector)
}

type Service struct {
	name   string
	routes []*Route
}

func NewService(name string) *Service {
	return &Service{name: name}
}

func (s *Service) Route(name string) *Route {
	route := newRoute(name)
	s.routes = append(s.routes, route)
	return route
}

type Route struct {
	name              string
	description       string
	path              string
	httpMethod        string
	streamingResponse bool
	requestType       reflect.Type
	responseType      reflect.Type
}

func newRoute(name string) *Route {
	return &Route{
		name: name,
	}
}

func (r *Route) Method(method, path string) *Route {
	r.httpMethod = method
	r.path = path
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

func (r *Route) Describe(text string) *Route {
	r.description = text
	return r
}

func (r *Route) Request(req interface{}) *Route {
	r.requestType = reflect.TypeOf(req)
	return r
}

func (r *Route) Response(resp interface{}) *Route {
	r.responseType = reflect.TypeOf(resp)
	return r
}

func (r *Route) Streaming() *Route {
	r.streamingResponse = true
	return r
}
