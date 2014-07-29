package schema

import "reflect"

type Schema struct {
	Name   string
	Routes []*Route
}

type Route struct {
	Name              string
	Description       string
	Path              string
	HTTPMethod        string
	StreamingResponse bool
	RequestType       reflect.Type
	ResponseType      reflect.Type
	QueryType         reflect.Type
	PathType          reflect.Type
	SuccessStatus     int
}
