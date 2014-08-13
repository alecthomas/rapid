package rapid

import (
	"reflect"

	"github.com/alecthomas/rapid/schema"
)

type Definition struct {
	name   string
	Schema *schema.Schema
}

// Define a new service.
func Define(name string) *Definition {
	return &Definition{
		Schema: &schema.Schema{
			Name: name,
		},
	}
}

func (d *Definition) Description(text string) *Definition {
	d.Schema.Description = text
	return d
}

func (d *Definition) Example(text string) *Definition {
	d.Schema.Example = text
	return d
}

func (d *Definition) Version(version string) *Definition {
	d.Schema.Version = version
	return d
}

func (d *Definition) Route(name string) *Route {
	route := newRoute(name)
	d.Schema.Routes = append(d.Schema.Routes, route.model)
	return route
}

type Route struct {
	model *schema.Route
}

func newRoute(name string) *Route {
	return &Route{
		model: &schema.Route{
			Name: name,
		}}
}

// Method explicitly sets the HTTP method for a route.
func (r *Route) Method(method, path string) *Route {
	r.model.Method = method
	r.model.Path = path
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

// Hidden hides a route from API dumps.
func (r *Route) Hidden() *Route {
	r.model.Hidden = true
	return r
}

// Description of the route.
func (r *Route) Description(text string) *Route {
	r.model.Description = text
	return r
}

func (r *Route) Example(text string) *Route {
	r.model.Example = text
	return r
}

// Query sets the type used to decode a request's query parameters. Each
// parameter is deserialized into the corresponding parameter using
// gorilla/schema.
func (r *Route) Query(query interface{}) *Route {
	r.model.QueryType = reflect.TypeOf(query)
	return r
}

// Path sets the type used to decode a request's path parameters. Each
// parameter is deserialized into the corresponding parameter using
// gorilla/schema.
func (r *Route) Path(params interface{}) *Route {
	r.model.PathType = reflect.TypeOf(params)
	return r
}

func (r *Route) Request(req interface{}) *Route {
	r.model.RequestType = reflect.TypeOf(req)
	return r
}

// Response adds a response definition.
func (r *Route) Response(status int, resp interface{}) *Route {
	return r.DefineResponse(&schema.Response{
		Status: status,
		Type:   reflect.TypeOf(resp),
	})
}

func (r *Route) DefineResponse(response *schema.Response) *Route {
	r.model.Responses = append(r.model.Responses, response)
	return r
}

// Streaming specifies that an endpoint returns a chunked streaming response
// (chan <type>, chan error).
func (r *Route) Streaming() *Route {
	r.model.StreamingResponse = true
	return r
}
