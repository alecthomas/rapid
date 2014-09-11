package rapid

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/alecthomas/rapid/schema"
)

type definition struct {
	model *schema.Schema
}

// Define a new service.
func Define(name string) *definition {
	return &definition{
		model: &schema.Schema{
			Name: name,
		},
	}
}

// Description of the schema.
func (d *definition) Description(text string) *definition {
	d.model.Description = text
	return d
}

// Example of using the schema.
func (d *definition) Example(text string) *definition {
	d.model.Example = text
	return d
}

func (d *definition) Version(version string) *definition {
	d.model.Version = version
	return d
}

func (d *definition) Resource(name, path string) *resource {
	r := &resource{&schema.Resource{
		Name: name,
		Path: path,
	}}
	d.model.Resources = append(d.model.Resources, r.model)
	return r
}

// Route adds a new route to the / resource.
func (d *definition) Route(name, path string) *route {
	// Try and find a resource.
	var res *resource
	parts := strings.Split(path, "/")
seek:
	for i := len(parts) - 1; i >= 0; i-- {
		seek := strings.Join(parts[:i], "/")
		for _, r := range d.model.Resources {
			if r.Path == seek {
				res = &resource{r}
				break seek
			}
		}
	}
	if res == nil {
		res = d.Resource(name, path)
	}
	return res.Route(name, path)
}

// Build a RAPID schema.
func (d *definition) Build() *schema.Schema {
	for _, resource := range d.model.Resources {
		for _, route := range resource.Routes {
			if route.Method == "" {
				panic(fmt.Sprintf("route %s has not specified a HTTP method", route.Name))
			}
			if route.Path == "" {
				panic(fmt.Sprintf("route %s with empty path", route.Name))
			}
			if !strings.HasPrefix(route.Path, resource.Path) {
				panic(fmt.Sprintf("route %s is not under resource %s", route, resource.Path))
			}
			// Check if different 200 responses have different response types. This is not supported.
			successful := false
			var okType reflect.Type
			for _, response := range route.Responses {
				if response.Status >= 200 && response.Status <= 299 {
					if successful && okType != response.Type {
						panic(fmt.Sprintf("multiple 2xx responses with differing types for %s", route))
					}
					okType = response.Type
					successful = true
				}
			}
			if !successful {
				if route.Method == "GET" {
					panic(fmt.Sprintf("no successful responses defined for %s", route))
				}
				route.Responses = append(route.Responses, &schema.Response{
					Status:      http.StatusNoContent,
					ContentType: "application/json",
				})
			}
		}
	}
	return d.model
}

type resource struct {
	model *schema.Resource
}

// Description of resource.
func (r *resource) Description(description string) *resource {
	r.model.Description = description
	return r
}

// Route adds a new route to this resource.
func (r *resource) Route(name, path string) *route {
	rt := newRoute(name, path)
	r.model.Routes = append(r.model.Routes, rt.model)
	return rt
}

type route struct {
	model *schema.Route
}

func newRoute(name, path string) *route {
	return &route{
		model: &schema.Route{
			Name: name,
			Path: path,
		}}
}

// Method explicitly sets the HTTP method for a route.
func (r *route) Method(method string) *route {
	r.model.Method = method
	return r
}

// Any matches any HTTP method.
func (r *route) Any() *route {
	return r.Method("")
}

func (r *route) Post() *route {
	return r.Method("POST")
}

func (r *route) Get() *route {
	return r.Method("GET")
}

func (r *route) Put() *route {
	return r.Method("PUT")
}

func (r *route) Delete() *route {
	return r.Method("DELETE")
}

func (r *route) Options() *route {
	return r.Method("OPTIONS")
}

// Hidden hides a route from API dumps.
func (r *route) Hidden() *route {
	r.model.Hidden = true
	return r
}

// Description of the route.
func (r *route) Description(text string) *route {
	r.model.Description = text
	return r
}

func (r *route) Example(text string) *route {
	r.model.Example = text
	return r
}

// Query sets the type used to decode a request's query parameters. Each
// parameter is deserialized into the corresponding parameter using
// gorilla/schema.
func (r *route) Query(query interface{}) *route {
	r.model.QueryType = reflect.TypeOf(query)
	return r
}

// Path sets the type used to decode a request's path parameters. Each
// parameter is deserialized into the corresponding parameter using
// gorilla/schema.
func (r *route) Path(params interface{}) *route {
	r.model.PathType = reflect.TypeOf(params)
	return r
}

func (r *route) Request(req interface{}) *route {
	r.model.RequestType = reflect.TypeOf(req)
	return r
}

func (r *route) Response(status int, typ interface{}) *route {
	return r.Responses(Response(status, typ))
}

func (r *route) Responses(responses ...*response) *route {
	for _, resp := range responses {
		r.model.Responses = append(r.model.Responses, resp.model)
	}
	return r
}

type response struct {
	model *schema.Response
}

func Response(status int, typ interface{}) *response {
	var t reflect.Type
	if typ != nil {
		t = reflect.TypeOf(typ)
	}
	return &response{
		&schema.Response{
			Status:      status,
			Type:        t,
			ContentType: "application/json",
		},
	}
}

func (r *response) Description(description string) *response {
	r.model.Description = description
	return r
}

func (r *response) ContentType(ct string) *response {
	r.model.ContentType = ct
	return r
}

func (r *response) Streaming() *response {
	r.model.Streaming = true
	return r
}
