package rapid

import (
	"reflect"
	"strings"
)

type Schema struct {
	Schema string `json:"$schema,omitempty"`
	Ref    string `json:"$ref,omitempty"`

	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Type        string             `json:"type,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Media       *SchemaMedia       `json:"media,omitempty"`
	Links       []*SchemaLink      `json:"links,omitempty"`
}

type SchemaLink struct {
	Title            string  `json:"title,omitempty"`
	Rel              string  `json:"rel,omitempty"`
	Href             string  `json:"href,omitempty"`
	Schema           *Schema `json:"schema,omitempty"`
	TargetSchema     *Schema `json:"targetSchema,omitempty"`
	Method           string  `json:"method,omitempty"`
	EncType          string  `json:"encType,omitempty"`
	MediaType        string  `json:"mediaType,omitempty"`
	TransferEncoding string  `json:"transferEncoding,omitempty"`
}

type SchemaMedia struct {
	BinaryEncoding string `json:"binaryEncoding,omitempty"`
	Type           string `json:"type,omitempty"`
}

// SchemaFrom builds a JSON schema v4 from a Go type.
func SchemaFrom(title string, prototype interface{}) *Schema {
	t := reflect.Indirect(reflect.ValueOf(prototype)).Type()
	s := schemaFromReflectedType(title, t)
	s.Schema = "http://json-schema.org/draft-04/schema#"
	s.Title = s.Description
	s.Description = ""
	return s
}

func SchemaFromService(service *Service) *Schema {
	s := &Schema{
		Title: service.Name,
	}
	for _, route := range service.Routes {
		transferEncoding := ""
		if route.StreamingResponse {
			transferEncoding = "chunked"
		}
		l := &SchemaLink{
			Rel:              route.Name,
			Href:             route.Path,
			Method:           route.HTTPMethod,
			Schema:           schemaFromReflectedType("", route.RequestType),
			TargetSchema:     schemaFromReflectedType("", route.ResponseType),
			TransferEncoding: transferEncoding,
		}
		if l.Schema != nil && l.EncType != "" {
			l.EncType = "application/json"
		}
		if l.TargetSchema != nil && l.MediaType != "" {
			l.MediaType = "application/json"
		}
		s.Links = append(s.Links, l)
	}
	return s
}

func schemaFromReflectedType(description string, t reflect.Type) *Schema {
	if t == nil {
		return nil
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	jt := schemaType(t)
	s := &Schema{
		Type:        jt,
		Description: description,
	}

	switch jt {
	case "object":
		s.Properties = make(map[string]*Schema)
		s.Required = []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name, options := parseTags(f.Tag.Get("json"))
			if name == "-" {
				name = f.Name
			}
			description := f.Tag.Get("description")
			ss := schemaFromReflectedType(description, f.Type)
			if !options["omitempty"] {
				s.Required = append(s.Required, name)
			}
			s.Properties[name] = ss
		}

	case "array":
		s.Items = schemaFromReflectedType("", t.Elem())

	case "number", "string":

	default:
		panic("unsupported type " + jt)
	}
	return s
}

func schemaType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"

	case reflect.String:
		return "string"

	case reflect.Slice, reflect.Array:
		return "array"

	case reflect.Struct:
		return "object"
	}
	panic("unsupported type " + t.Kind().String())
}

func parseTags(tag string) (string, map[string]bool) {
	parts := strings.Split(tag, ",")
	name := parts[0]
	if name == "" {
		name = "-"
	}
	options := map[string]bool{}
	for _, p := range parts[1:] {
		options[p] = true
	}
	return name, options
}
