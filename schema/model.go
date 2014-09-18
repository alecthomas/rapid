package schema

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

var (
	pathTransform = regexp.MustCompile(`{((\w+)(?::((?:\\.|[^}])+))?)}`)
)

type Routes []*Route

func (r Routes) Len() int           { return len(r) }
func (r Routes) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r Routes) Less(i, j int) bool { return r[i].Path < r[j].Path }

type Schema struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Example     string      `json:"example"`
	Version     string      `json:"version,omitempty"`
	Resources   []*Resource `json:"resources"`
}

func (s *Schema) ResourceByPath(path string) *Resource {
	for _, r := range s.Resources {
		if r.Path == path {
			return r
		}
	}
	return nil
}

type Resource struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Routes      Routes `json:"routes"`
}

func (r *Resource) SimplifyPath() string {
	return simplifiedPath(r.Path)
}

func (r *Resource) Hidden() bool {
	for _, route := range r.Routes {
		if !route.Hidden {
			return false
		}
	}
	return true
}

type Route struct {
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Example     string       `json:"example,omitempty"`
	Path        string       `json:"path"`
	Method      string       `json:"method"`
	RequestType reflect.Type `json:"request_type"`
	Responses   []*Response  `json:"responses"`
	QueryType   reflect.Type `json:"query_type"`
	PathType    reflect.Type `json:"path_type"`
	SecuredBy   []string     `json:"secured_by"`

	Hidden bool `json:"-"` // A hint that this should be hidden from public API descriptions.
}

func (r *Route) String() string {
	return fmt.Sprintf("%s %s", r.Method, r.Path)
}

// DefaultResponse returns the first response with a 2xx status code, assumed
// to be the default response.
func (r *Route) DefaultResponse() *Response {
	for _, response := range r.Responses {
		if response.Status >= 200 && response.Status <= 299 {
			return response
		}
	}
	return nil
}

type Response struct {
	Status      int          `json:"status"`
	Description string       `json:"description"`
	ContentType string       `json:"content_type,omitempty"`
	Type        reflect.Type `json:"type"`
	Streaming   bool         `json:"streaming,omitempty"`
}

func setStructType(types map[reflect.Type]struct{}, t reflect.Type) {
	if t == nil {
		return
	}
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Struct {
		types[t] = struct{}{}
	}
}

// Types returns a set of all referenced types used in the schema.
func (s *Schema) Types() []reflect.Type {
	types := map[reflect.Type]struct{}{}
	for _, resource := range s.Resources {
		for _, route := range resource.Routes {
			setStructType(types, route.RequestType)
			for _, r := range route.Responses {
				setStructType(types, r.Type)
			}
			setStructType(types, route.QueryType)
			setStructType(types, route.PathType)
		}
	}

	out := []reflect.Type{}
	for model := range types {
		out = append(out, model)
	}
	return out
}

// CompilePath compiles a path into a regex and its named parameters.
func (r *Route) CompilePath() (*regexp.Regexp, []string) {
	routePattern := "^" + r.Path + "$"
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

func (r *Route) SimplifyPath() string {
	return simplifiedPath(r.Path)
}

func (r *Route) InterpolatePath(args ...interface{}) string {
	return InterpolatePath(r.Path, args...)
}

func InterpolatePath(path string, args ...interface{}) string {
	out := path
	for i, match := range pathTransform.FindAllStringSubmatch(path, -1) {
		v := fmt.Sprintf("%v", args[i])
		out = strings.Replace(out, match[0], v, 1)
	}
	return out
}

func simplifiedPath(path string) string {
	for _, match := range pathTransform.FindAllStringSubmatch(path, -1) {
		path = strings.Replace(path, match[0], "{"+match[2]+"}", 1)
	}
	return path
}
