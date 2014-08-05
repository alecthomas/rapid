package schema

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
)

type jsonSchema struct {
	Routes []*jsonRoute `json:"routes"`
	*Schema
}

type jsonRoute struct {
	RequestType  *jsonType `json:"request_type"`
	ResponseType *jsonType `json:"response_type"`
	QueryType    *jsonType `json:"query_type"`
	PathType     *jsonType `json:"path_type"`
	*Route
}

type jsonType struct {
	Kind       string      `json:"kind"`
	Name       string      `json:"name,omitempty"`
	Fields     []*jsonType `json:"fields,omitempty"`
	Key        *jsonType   `json:"key,omitempty"`
	Value      *jsonType   `json:"value,omitempty"`
	Annotation string      `json:"annotation,omitempty"`
}

// func (j *jsonType) MarshalJSON() ([]byte, error) {

// }

func SchemaToJSON(s *Schema, w io.Writer) error {
	schema := &jsonSchema{
		Schema: s,
	}
	for _, route := range s.Routes {
		schema.Routes = append(schema.Routes, routeToJSON(route))
	}
	return json.NewEncoder(w).Encode(schema)
}

func routeToJSON(r *Route) *jsonRoute {
	return &jsonRoute{
		Route:        r,
		RequestType:  typeToJSON(r.RequestType),
		ResponseType: typeToJSON(r.ResponseType),
		QueryType:    typeToJSON(r.QueryType),
		PathType:     typeToJSON(r.PathType),
	}
}

// struct -> {"name": <name>, "fields": {...}}
func typeToJSON(t reflect.Type) *jsonType {
	if t == nil {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		fields := []*jsonType{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			j := typeToJSON(f.Type)
			j.Annotation = string(f.Tag)
			j.Name = f.Name
			fields = append(fields, j)
		}
		return &jsonType{
			Kind:   "struct",
			Name:   t.Name(),
			Fields: fields,
		}

	case reflect.Ptr:
		return typeToJSON(t.Elem())

	case reflect.Interface:
		return &jsonType{Kind: "any"}

	case reflect.String:
		return &jsonType{Kind: "string"}

	case reflect.Int:
		return &jsonType{Kind: "int"}

	case reflect.Int8:
		return &jsonType{Kind: "int8"}

	case reflect.Int16:
		return &jsonType{Kind: "int16"}

	case reflect.Int32:
		return &jsonType{Kind: "int32"}

	case reflect.Int64:
		return &jsonType{Kind: "int64"}

	case reflect.Uint:
		return &jsonType{Kind: "uint"}

	case reflect.Uint8:
		return &jsonType{Kind: "uint8"}

	case reflect.Uint16:
		return &jsonType{Kind: "uint16"}

	case reflect.Uint32:
		return &jsonType{Kind: "uint32"}

	case reflect.Uint64:
		return &jsonType{Kind: "uint64"}

	case reflect.Float32:
		return &jsonType{Kind: "float32"}

	case reflect.Float64:
		return &jsonType{Kind: "float64"}

	case reflect.Slice:
		return &jsonType{Kind: "slice", Value: typeToJSON(t.Elem())}

	case reflect.Map:
		return &jsonType{Kind: "map", Key: typeToJSON(t.Key()), Value: typeToJSON(t.Elem())}
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}
