package schema

import (
	"fmt"
	"reflect"
)

type PublicSchema struct {
	Routes []*PublicRoute `json:"routes"`
	*Schema
}

type PublicRoute struct {
	RequestType  *PublicType `json:"request_type"`
	ResponseType *PublicType `json:"response_type"`
	QueryType    *PublicType `json:"query_type"`
	PathType     *PublicType `json:"path_type"`
	*Route
}

type PublicType struct {
	Kind       string        `json:"kind"`
	Name       string        `json:"name,omitempty"`
	Fields     []*PublicType `json:"fields,omitempty"`
	Key        *PublicType   `json:"key,omitempty"`
	Value      *PublicType   `json:"value,omitempty"`
	Annotation string        `json:"annotation,omitempty"`
}

// SchemaToPublic converts an internal Go Schema to a structure that is safe
// for publication (as JSON, etc.).
func SchemaToPublic(s *Schema) *PublicSchema {
	schema := &PublicSchema{
		Schema: s,
	}
	for _, route := range s.Routes {
		schema.Routes = append(schema.Routes, routeToPublic(route))
	}
	return schema
}

func routeToPublic(r *Route) *PublicRoute {
	return &PublicRoute{
		Route:        r,
		RequestType:  typeToPublic(r.RequestType),
		ResponseType: typeToPublic(r.ResponseType),
		QueryType:    typeToPublic(r.QueryType),
		PathType:     typeToPublic(r.PathType),
	}
}

// struct -> {"name": <name>, "fields": {...}}
func typeToPublic(t reflect.Type) *PublicType {
	if t == nil {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		fields := []*PublicType{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			j := typeToPublic(f.Type)
			j.Annotation = string(f.Tag)
			j.Name = f.Name
			fields = append(fields, j)
		}
		return &PublicType{
			Kind:   "struct",
			Name:   t.Name(),
			Fields: fields,
		}

	case reflect.Ptr:
		return typeToPublic(t.Elem())

	case reflect.Interface:
		return &PublicType{Kind: "any"}

	case reflect.String:
		return &PublicType{Kind: "string"}

	case reflect.Int:
		return &PublicType{Kind: "int"}

	case reflect.Int8:
		return &PublicType{Kind: "int8"}

	case reflect.Int16:
		return &PublicType{Kind: "int16"}

	case reflect.Int32:
		return &PublicType{Kind: "int32"}

	case reflect.Int64:
		return &PublicType{Kind: "int64"}

	case reflect.Uint:
		return &PublicType{Kind: "uint"}

	case reflect.Uint8:
		return &PublicType{Kind: "uint8"}

	case reflect.Uint16:
		return &PublicType{Kind: "uint16"}

	case reflect.Uint32:
		return &PublicType{Kind: "uint32"}

	case reflect.Uint64:
		return &PublicType{Kind: "uint64"}

	case reflect.Float32:
		return &PublicType{Kind: "float32"}

	case reflect.Float64:
		return &PublicType{Kind: "float64"}

	case reflect.Slice:
		return &PublicType{Kind: "slice", Value: typeToPublic(t.Elem())}

	case reflect.Map:
		return &PublicType{Kind: "map", Key: typeToPublic(t.Key()), Value: typeToPublic(t.Elem())}
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}
