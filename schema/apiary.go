package schema

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"text/template"
)

func buildSampleType(t reflect.Type) reflect.Value {
	if t.Kind() == reflect.Slice {
		s := reflect.MakeSlice(t, 0, 1)
		s = reflect.Append(s, buildSampleType(t.Elem()))
		return s
	} else if t.Kind() == reflect.Ptr {
		return reflect.New(t.Elem())
	}
	return reflect.New(t)
}

var (
	apiaryFuncs = template.FuncMap{
		"json": func(t reflect.Type) string {
			v := buildSampleType(t)
			b, _ := json.MarshalIndent(v.Interface(), "", "  ")
			return string(b)
		},
		"statusFor": func(method string, status int) int {
			if status == 0 {
				if method == "POST" {
					return http.StatusCreated
				}
				return http.StatusOK
			}
			return status
		},
		// {{.Text|indent 4}}
		"indent": func(indent int, text string) string {
			i := strings.Repeat(" ", indent)
			return strings.Join(strings.Split(text, "\n"), "\n"+i)
		},
	}
	apiaryTemplate = template.Must(template.New("apiary").Funcs(apiaryFuncs).Parse(
		`FORMAT: 1A

# {{.Name}}{{if .Description}}

{{.Description}}{{end}}
{{range .Routes}}
# {{.Method}} {{.Path}}

{{.Name}}{{if .Description}} - {{.Description}}{{end}}
{{if .RequestType}}
+ Request (application/json)

            {{.RequestType|json|indent 12}}
{{end}}
+ Response {{.SuccessStatus|statusFor .Method}} (application/json){{if .ResponseType}}{{if .StreamingResponse}}
    + Headers

            Content-Encoding: chunked
{{end}}
    + Body

            {{.ResponseType|json|indent 12}}
{{end}}
{{end}}
`))
)

func SchemaToApiary(schema *Schema, w io.Writer) error {
	sort.Sort(schema.Routes)
	return apiaryTemplate.Execute(w, schema)
}
