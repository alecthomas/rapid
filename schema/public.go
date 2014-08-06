package schema

import (
	"crypto/sha1"
	"fmt"
	"io"
	"reflect"
)

type PublicSchema struct {
	Routes []*PublicRoute `json:"routes"`
	*Schema
	Types map[string]*PublicType
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

type schemaToPublicContext map[string]*PublicType

// ShortestPrefix transforms package hashes to their shortest unique prefix.
func (s schemaToPublicContext) ShortestPrefix() schemaToPublicContext {
	out := schemaToPublicContext{}
	prefix := 3
	uniques := map[string]bool{}
	for k := range s {
		uniques[k[:40]] = true
	}
	for ; prefix < 40; prefix++ {
		keys := map[string]int{}
		for k := range s {
			k = k[:prefix]
			keys[k]++
		}
		if len(keys) == len(uniques) {
			break
		}
	}

	for k, v := range s {
		out[k[:prefix]+k[40:]] = v
	}
	return out
}

// SchemaToPublic converts an internal Go Schema to a structure that is safe
// for publication (as JSON, etc.).
func SchemaToPublic(s *Schema) *PublicSchema {
	context := schemaToPublicContext{}
	schema := &PublicSchema{
		Schema: s,
	}
	for _, route := range s.Routes {
		schema.Routes = append(schema.Routes, routeToPublic(context, route))
	}
	schema.Types = context.ShortestPrefix()
	return schema
}

func routeToPublic(context schemaToPublicContext, r *Route) *PublicRoute {
	return &PublicRoute{
		Route:        r,
		RequestType:  typeToPublic(context, r.RequestType),
		ResponseType: typeToPublic(context, r.ResponseType),
		QueryType:    typeToPublic(context, r.QueryType),
		PathType:     typeToPublic(context, r.PathType),
	}
}

// struct -> {"name": <name>, "fields": {...}}
func typeToPublic(context schemaToPublicContext, t reflect.Type) (out *PublicType) {
	if t == nil {
		return nil
	}

	hash := sha1.New()
	_, _ = io.WriteString(hash, t.PkgPath())
	key := fmt.Sprintf("%x.%s", hash.Sum(nil), t.Name())

	if _, ok := context[key]; ok {
		return &PublicType{Kind: "ref", Name: key}
	}

	switch t.Kind() {
	case reflect.Struct:
		fields := []*PublicType{}
		pt := &PublicType{
			Kind: "struct",
			Name: t.Name(),
		}
		context[key] = pt
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			j := typeToPublic(context, f.Type)
			j.Annotation = string(f.Tag)
			j.Name = f.Name
			fields = append(fields, j)
		}
		pt.Fields = fields
		return &PublicType{Kind: "ref", Name: key}

	case reflect.Ptr:
		return typeToPublic(context, t.Elem())

	case reflect.Interface:
		return &PublicType{Kind: "any"}

	case reflect.Bool:
		return &PublicType{Kind: "bool"}

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
		return &PublicType{Kind: "slice", Value: typeToPublic(context, t.Elem())}

	case reflect.Map:
		return &PublicType{Kind: "map", Key: typeToPublic(context, t.Key()), Value: typeToPublic(context, t.Elem())}
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}
