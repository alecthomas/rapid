package schema

import (
	"encoding/json"
	"io"
	"reflect"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/jsonschema"
	"gopkg.in/yaml.v1"
)

var (
	timeType = reflect.TypeOf(time.Time{})
	varRegex = regexp.MustCompile(`{([^}]+)}`)
)

type ramlTypeMap map[string]reflect.Type

type rmap map[interface{}]interface{}

type ramlNestedRoute struct {
	routes Routes
	nested map[string]*ramlNestedRoute
}

func buildRoutes(rnr *ramlNestedRoute, parts []string, r *Route) {
	if r.Hidden {
		return
	}
	if len(parts) == 0 {
		rnr.routes = append(rnr.routes, r)
		return
	}
	key := "/" + parts[0]
	sr, ok := rnr.nested[key]
	if !ok {
		sr = &ramlNestedRoute{
			nested: map[string]*ramlNestedRoute{},
		}
		rnr.nested[key] = sr
	}
	buildRoutes(sr, parts[1:], r)
}

func SchemaToRAML(url string, s *Schema, w io.Writer) error {
	title := s.Name
	if s.Description != "" {
		title = s.Name + " - " + s.Description
	}
	y := rmap{
		"baseUri":   url,
		"mediaType": "application/json",
		"title":     title,
		// "securitySchemes": []rmap{
		// 	rmap{
		// 		"basic": rmap{
		// 			"type": "Basic Authentication",
		// 		},
		// 	},
		// },
		// "securedBy": []string{
		// 	"basic",
		// },
		// https://github.com/raml-org/raml-js-parser/issues/108
		// "displayName": s.Name,
	}
	rnr := &ramlNestedRoute{
		nested: map[string]*ramlNestedRoute{},
	}
	for _, res := range s.Resources {
		for _, r := range res.Routes {
			parts := strings.Split(r.SimplifyPath(), "/")[1:]
			buildRoutes(rnr, parts, r)
		}
	}

	typeMap := ramlTypeMap{}
	schemas := []rmap{}
	for _, res := range s.Resources {
		for _, r := range res.Routes {
			collectTypes(typeMap, r.RequestType)
			for _, rs := range r.Responses {
				collectTypes(typeMap, rs.Type)
			}
		}
	}
	for name, t := range typeMap {
		b, err := json.MarshalIndent(jsonschema.Reflect(reflect.New(t).Interface()), "", "  ")
		if err != nil {
			return err
		}
		schemas = append(schemas, rmap{name: string(b)})
	}
	if len(schemas) > 0 {
		y["schemas"] = schemas
	}

	for k, v := range routesToRAML("", rnr) {
		y[k] = v
	}
	b, err := yaml.Marshal(y)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte("#%RAML 0.8\n"))
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

func collectTypes(typeMap ramlTypeMap, t reflect.Type) {
	if t == nil || t == timeType {
		return
	}
	if _, ok := typeMap[t.Name()]; ok {
		return
	}

	switch t.Kind() {
	case reflect.Struct:
		typeMap[t.Name()] = t
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if isFirstLower(f.Name) {
				continue
			}
			collectTypes(typeMap, f.Type)
		}

	case reflect.Ptr, reflect.Slice:
		collectTypes(typeMap, t.Elem())

	case reflect.Map:
		collectTypes(typeMap, t.Key())
		collectTypes(typeMap, t.Elem())
	}
}

func ramlSchemaForType(t reflect.Type) rmap {
	if t == nil {
		return rmap{}
	}
	var schema string
	switch t.Kind() {
	case reflect.Ptr:
		return ramlSchemaForType(t.Elem())

	case reflect.Struct:
		schema = t.Name()

	default:
		b, err := json.MarshalIndent(jsonschema.ReflectFromType(t), "", "  ")
		if err != nil {
			panic(err)
		}
		schema = string(b)
	}
	return rmap{
		"schema":  schema,
		"example": makeRAMLExample(t),
	}
}

func routesToRAML(path string, r *ramlNestedRoute) rmap {
	out := rmap{}
	if len(r.routes) > 0 {
		route := r.routes[0]
		if route.PathType != nil {
			vars := varRegex.FindAllStringSubmatch(path, -1)
			ip := structToRAMLParams(route.PathType, true)
			op := rmap{}
			for _, vn := range vars {
				if _, ok := ip[vn[1]]; ok {
					op[vn[1]] = ip[vn[1]]
				}
			}
			out["uriParameters"] = op
		}
	}

	for _, r := range r.routes {
		rm := rmap{
			"responses": rmap{},
		}

		// Responses
		responseMap := rmap{}
		rm["responses"] = responseMap
		if r.QueryType != nil {
			rm["queryParameters"] = structToRAMLParams(r.QueryType, false)
		}
		for _, response := range r.Responses {
			ct := response.ContentType
			if ct == "" {
				ct = "application/json"
			}
			rresp := ramlSchemaForType(response.Type)
			rrm := rmap{
				"body": rmap{
					ct: rresp,
				},
			}
			description := response.Description
			if response.Streaming {
				description = "Streaming response."
				rrm["headers"] = rmap{
					"Content-Encoding": rmap{
						"type": "string",
					},
				}
			}
			if description != "" {
				rrm["description"] = description
			}
			responseMap[response.Status] = rrm
		}

		// FIXME: This should work:
		// https://github.com/raml-org/raml-js-parser/issues/108
		// rm["displayName"] = r.Name
		if r.Description == "" {
			rm["description"] = r.Name
		} else {
			rm["description"] = r.Name + " - " + r.Description
		}
		if r.RequestType != nil {
			rreq := ramlSchemaForType(r.RequestType)
			if r.Example != "" {
				rreq["example"] = r.Example
			}
			rm["body"] = rmap{
				"application/json": rreq,
			}
		}
		out[strings.ToLower(r.Method)] = rm
	}
	for path, sr := range r.nested {
		out[path] = routesToRAML(path, sr)
	}
	return out
}

type cycleMap map[reflect.Type]bool

func makeRAMLExample(t reflect.Type) string {
	cycles := cycleMap{}
	b, err := json.MarshalIndent(makeRAMLExampleValue(cycles, t).Interface(), "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b)
}

func makeRAMLExampleValue(cycles cycleMap, t reflect.Type) reflect.Value {
	switch t.Kind() {
	case reflect.Ptr:
		return makeRAMLExampleValue(cycles, t.Elem()).Addr()

	case reflect.Struct:
		v := reflect.New(t).Elem()
		if cycles[t] {
			return v
		}
		cycles[t] = true
		for i := 0; i < v.NumField(); i++ {
			ft := t.Field(i)
			if isFirstLower(ft.Name) {
				continue
			}
			f := v.Field(i)
			f.Set(makeRAMLExampleValue(cycles, ft.Type))
		}
		return v

	case reflect.Slice:
		v := reflect.MakeSlice(t, 0, 0)
		v = reflect.Append(v, makeRAMLExampleValue(cycles, t.Elem()))
		return v

	case reflect.Map:
		v := reflect.MakeMap(t)
		v.SetMapIndex(makeRAMLExampleValue(cycles, t.Key()), makeRAMLExampleValue(cycles, t.Elem()))
		return v

	default:
		return reflect.New(t).Elem()
	}
}

// func makeRAMLExampleValue(cycles map[reflect.Type]bool, t reflect.Type) reflect.Value {
// 	switch t.Kind() {
// 	case reflect.Ptr:
// 		return makeRAMLExampleValue(cycles, t.Elem()).Addr()

// 	case reflect.Slice:
// 		return reflect.MakeSlice(t, 0, 0)

// 	case reflect.Struct:
// 		return fillRAMLExampleValue(cycles, reflect.New(t).Elem())

// 	default:
// 		return reflect.New(t).Elem()
// 	}
// }

// func fillRAMLExampleValue(cycles cycleMap, v reflect.Value) reflect.Value {
// 	switch v.Kind() {
// 	case reflect.Ptr:
// 		fillRAMLExampleValue(cycles, v.Elem())

// 	case reflect.Slice:
// 		return reflect.Append(v, makeRAMLExampleValue(cycles, v.Type().Elem()))

// 	case reflect.Struct:
// 		if cycles[v.Type()] {
// 			return reflect.Zero(v.Type())
// 		}
// 		cycles[v.Type()] = true
// 		for i := 0; i < v.NumField(); i++ {
// 			f := v.Field(i)
// 			if f.CanSet() && (f.Kind() == reflect.Ptr || f.Kind() == reflect.Slice) {
// 				f.Set(fillRAMLExampleValue(cycles, makeRAMLExampleValue(cycles, f.Type())))
// 			}
// 		}
// 	}
// 	return v
// }

func structToRAMLParams(t reflect.Type, required bool) rmap {
	out := rmap{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name, _ := parseTag(f)
		rm := typeToRAML(f.Type)
		if required {
			rm["required"] = true
		}
		out[name] = rm
	}
	return out
}

func typeToRAML(t reflect.Type) rmap {
	switch t.Kind() {
	case reflect.Struct:
		switch t {
		case timeType:
			return rmap{"type": "date"}
		}

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rmap{"type": "integer"}

	case reflect.Float32, reflect.Float64:
		return rmap{"type": "number"}

	case reflect.Bool:
		return rmap{"type": "boolean"}

	case reflect.String:
		return rmap{"type": "string"}

	case reflect.Ptr:
		return typeToRAML(t.Elem())
	}
	panic("unsupported type " + t.String())
}

func isFirstLower(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLower(r)
}

func parseTag(f reflect.StructField) (name string, required bool) {
	name = f.Name
	required = true
	json := f.Tag.Get("json")
	if json != "" {
		parts := strings.Split(json, ",")
		if parts[0] == "-" {
			name = ""
			return
		}
		name = parts[0]
		required = (len(parts) < 2 || parts[1] != "omitempty")
	}
	schema := f.Tag.Get("schema")
	if schema != "" {
		if name == "-" {
			name = ""
		} else {
			name = schema
		}
	}
	return
}
