package rapid

import "reflect"

type Definition struct {
	name   string
	routes []*Route
}

// Define a new service.
func Define(name string) *Definition {
	return &Definition{name: name}
}

func (s *Definition) Route(name string) *Route {
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
	queryType         reflect.Type
	pathType          reflect.Type
	successStatus     int
}

func newRoute(name string) *Route {
	return &Route{name: name}
}

// Method explicitly sets the HTTP method for a route.
func (r *Route) Method(method, path string) *Route {
	r.httpMethod = method
	r.path = path
	return r
}

// Any matches any HTTP method.
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

// Description of the route.
func (r *Route) Description(text string) *Route {
	r.description = text
	return r
}

// Query sets the type used to decode a request's query parameters. Each
// parameter is deserialized into the corresponding parameter using
// gorilla/schema.
func (r *Route) Query(query interface{}) *Route {
	r.queryType = reflect.TypeOf(query)
	return r
}

// Path sets the type used to decode a request's path parameters. Each
// parameter is deserialized into the corresponding parameter using
// gorilla/schema.
func (r *Route) Path(params interface{}) *Route {
	r.pathType = reflect.TypeOf(params)
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

// Streaming specifies that an endpoint returns a chunked streaming response
// (chan <type>, chan error).
func (r *Route) Streaming() *Route {
	r.streamingResponse = true
	return r
}

// Success overrides the status code to return for a successful (no error) response.
func (r *Route) SuccessStatus(status int) *Route {
	r.successStatus = status
	return r
}
