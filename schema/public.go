package schema

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

type PublicSchema struct {
	Routes []*PublicRoute `json:"routes"`
	*Schema
	Types map[string]*PublicType
}

type PublicRoute struct {
	RequestType  *PublicType `json:"request_type,omitempty"`
	ResponseType *PublicType `json:"response_type,omitempty"`
	QueryType    *PublicType `json:"query_type,omitempty"`
	PathType     *PublicType `json:"path_type,omitempty"`
	*Route
}

type concretePublicType struct {
	Type      string        `json:"type"`
	Name      string        `json:"name,omitempty"`
	Fields    []*PublicType `json:"fields,omitempty"`
	Key       *PublicType   `json:"key,omitempty"`
	Value     *PublicType   `json:"value,omitempty"`
	OmitEmpty bool          `json:"omit_empty,omitempty"`
}

type PublicType struct {
	*concretePublicType

	key string
	ref *PublicType
}

func newPublicType(kind string, key, value *PublicType) *PublicType {
	return &PublicType{
		concretePublicType: &concretePublicType{
			Type:  kind,
			Key:   key,
			Value: value,
		},
	}
}

type publicRefType struct {
	Name string `json:"name,omitempty"`
	Type string `json:"type"`
}

func newPublicTypeRef(ref *PublicType) *PublicType {
	return &PublicType{
		concretePublicType: &concretePublicType{
			Type: "ref",
			Name: ref.Name,
		},
		ref: ref,
	}
}

func (p *PublicType) MarshalJSON() ([]byte, error) {
	if p.ref != nil {
		return json.Marshal(&publicRefType{
			Type: p.ref.Type,
			Name: p.Name,
		})
	}
	return json.Marshal(p.concretePublicType)
}

type schemaToPublicContext map[string]*PublicType

// ShortestPrefix transforms package hashes to their shortest unique prefix.
func (s schemaToPublicContext) ShortestPrefix() schemaToPublicContext {
	out := schemaToPublicContext{}
	prefix := 0
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
		t := strings.TrimLeft(k[:prefix]+k[40:], ".")
		out[t] = v
		v.Type = t
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
		if route.Hidden {
			continue
		}
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

func isFirstLower(s string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return unicode.IsLower(r)
}

func isStruct(t reflect.Type) bool {
	return t.Kind() == reflect.Struct || (t.Kind() == reflect.Ptr && isStruct(t.Elem()))
}

func collectPublicFields(context schemaToPublicContext, t reflect.Type) []*PublicType {
	fields := []*PublicType{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		ft := f.Type
		for ft.Kind() == reflect.Ptr {
			ft = ft.Elem()
		}
		if f.Anonymous && ft.Kind() == reflect.Struct {
			fields = append(fields, collectPublicFields(context, ft)...)
			continue
		}
		if isFirstLower(f.Name) {
			continue
		}

		j := typeToPublic(context, f.Type)
		j.Name, j.OmitEmpty = parseTag(f)
		fields = append(fields, j)
	}
	return fields
}

func parseTag(f reflect.StructField) (name string, omitEmpty bool) {
	name = f.Name
	json := f.Tag.Get("json")
	if json != "" {
		parts := strings.Split(json, ",")
		if parts[0] == "-" {
			name = ""
			return
		} else {
			name = parts[0]
		}
		omitEmpty = (len(parts) > 1 && parts[1] == "omitempty")
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

func typeKey(t reflect.Type) string {
	hash := sha1.New()
	_, _ = io.WriteString(hash, t.PkgPath())
	key := fmt.Sprintf("%x.%s", hash.Sum(nil), t.Name())
	return key
}

// struct -> {"name": <name>, "fields": {...}}
func typeToPublic(context schemaToPublicContext, t reflect.Type) (out *PublicType) {
	if t == nil {
		return nil
	}

	if t.PkgPath() != "" && isFirstLower(t.Name()) {
		panic(fmt.Sprintf("can't export private type %s.%s", t.PkgPath(), t.Name()))
	}

	key := typeKey(t)
	if ref, ok := context[key]; ok {
		return newPublicTypeRef(ref)
	}

	switch t.Kind() {
	case reflect.Struct:
		pt := &PublicType{
			concretePublicType: &concretePublicType{
				Type: key,
			},
			key: key,
		}
		context[key] = pt
		fields := collectPublicFields(context, t)
		pt.Fields = fields
		return newPublicTypeRef(pt)

	case reflect.Ptr:
		return typeToPublic(context, t.Elem())

	case reflect.Interface:
		return newPublicType("any", nil, nil)

	case reflect.Bool:
		return newPublicType("bool", nil, nil)

	case reflect.String:
		return newPublicType("string", nil, nil)

	case reflect.Int:
		return newPublicType("int", nil, nil)

	case reflect.Int8:
		return newPublicType("int8", nil, nil)

	case reflect.Int16:
		return newPublicType("int16", nil, nil)

	case reflect.Int32:
		return newPublicType("int32", nil, nil)

	case reflect.Int64:
		return newPublicType("int64", nil, nil)

	case reflect.Uint:
		return newPublicType("uint", nil, nil)

	case reflect.Uint8:
		return newPublicType("uint8", nil, nil)

	case reflect.Uint16:
		return newPublicType("uint16", nil, nil)

	case reflect.Uint32:
		return newPublicType("uint32", nil, nil)

	case reflect.Uint64:
		return newPublicType("uint64", nil, nil)

	case reflect.Float32:
		return newPublicType("float32", nil, nil)

	case reflect.Float64:
		return newPublicType("float64", nil, nil)

	case reflect.Slice:
		return newPublicType("slice", nil, typeToPublic(context, t.Elem()))

	case reflect.Map:
		return newPublicType("map", typeToPublic(context, t.Key()), typeToPublic(context, t.Elem()))
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}
