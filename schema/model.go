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
	Name        string `json:"name"`
	Description string `json:"description"`
	Example     string `json:"example"`
	Version     string `json:"version,omitempty"`
	Routes      Routes `json:"routes"`
}

type Route struct {
	Name              string       `json:"name"`
	Description       string       `json:"description,omitempty"`
	Example           string       `json:"example,omitempty"`
	Path              string       `json:"path"`
	Method            string       `json:"method"`
	StreamingResponse bool         `json:"streaming_response,omitempty"`
	RequestType       reflect.Type `json:"request_type"`
	Responses         []*Response  `json:"responses"`
	QueryType         reflect.Type `json:"query_type"`
	PathType          reflect.Type `json:"path_type"`

	Hidden bool `json:"-"`
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

// Types returns a set of all references types used in the schema.
func (s *Schema) Types() []reflect.Type {
	types := map[reflect.Type]struct{}{}
	for _, route := range s.Routes {
		setStructType(types, route.RequestType)
		for _, r := range route.Responses {
			setStructType(types, r.Type)
		}
		setStructType(types, route.QueryType)
		setStructType(types, route.PathType)
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
	out := r.Path
	for _, match := range pathTransform.FindAllStringSubmatch(r.Path, -1) {
		out = strings.Replace(out, match[0], "{"+match[2]+"}", 1)
	}
	return out
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
