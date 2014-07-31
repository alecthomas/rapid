package schema

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"reflect"
	"sort"
	"strings"
	"text/template"
)

func goTypeReference(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Struct:
		return t.Name()

	case reflect.Ptr:
		return "*" + goTypeReference(t.Elem())

	case reflect.String:
		return "string"

	case reflect.Int:
		return "int"

	case reflect.Int8:
		return "int8"

	case reflect.Int16:
		return "int16"

	case reflect.Int32:
		return "int32"

	case reflect.Int64:
		return "int64"

	case reflect.Uint:
		return "uint"

	case reflect.Uint8:
		return "uint8"

	case reflect.Uint16:
		return "uint16"

	case reflect.Uint32:
		return "uint32"

	case reflect.Uint64:
		return "uint64"

	case reflect.Float32:
		return "float32"

	case reflect.Float64:
		return "float64"

	case reflect.Slice:
		return "[]" + goTypeReference(t.Elem())
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}

func goTypeDefinition(t reflect.Type) (name string, definition string) {
	switch t.Kind() {
	case reflect.Struct:
		out := &bytes.Buffer{}
		out.WriteString("struct {\n")
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fmt.Fprintf(out, "\t%s %s", f.Name, goTypeReference(f.Type))
			if f.Tag != "" {
				fmt.Fprintf(out, "\t`%s`", f.Tag)
			}
			fmt.Fprintf(out, "\n")
		}
		out.WriteString("}")
		return t.Name(), out.String()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "", ""

	case reflect.Ptr:
		return goTypeDefinition(t.Elem())

	default:
		return "", goTypeReference(t)
	}
}

func goPathTypeToParams(t reflect.Type) string {
	switch t.Kind() {
	case reflect.Struct:
		out := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := f.Tag.Get("schema")
			if name == "" {
				name = f.Name
			}
			out = append(out, fmt.Sprintf("%s %s", name, goTypeReference(f.Type)))
		}
		return strings.Join(out, ", ")

	case reflect.Ptr:
		return goPathTypeToParams(t.Elem())
	}
	panic("invalid path type")
}

func goPathNames(t reflect.Type) []string {
	if t == nil {
		return []string{}
	}
	switch t.Kind() {
	case reflect.Struct:
		out := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := f.Tag.Get("schema")
			if name == "" {
				name = f.Name
			}
			out = append(out, name)
		}
		return out

	case reflect.Ptr:
		return goPathNames(t.Elem())
	}
	panic("invalid path type")

}

func goTypeDecl(name string, t reflect.Type) string {
	switch t.Kind() {
	case reflect.Slice:
		return fmt.Sprintf("%s := []%s{}", name, goTypeReference(t.Elem()))

	case reflect.Struct:
		return fmt.Sprintf("%s := &%s{}", name, t.Name())

	case reflect.Ptr:
		return goTypeDecl(name, t.Elem())

	default:
		return fmt.Sprintf("var %s %s", name, goTypeReference(t))
	}
}

func goTypeRef(name string, t reflect.Type) string {
	if t == nil {
		return "nil"
	}
	switch t.Kind() {
	case reflect.Ptr:
		return name

	default:
		return "&" + name
	}
}

var (
	goTemplate = `package {{.Package}}

import (
	"github.com/alecthomas/rapid"
)

{{range $name, $definition := .Definitions}}
type {{$name}} {{$definition}}
{{end}}

{{if .Schema.Description}}// {{.Schema.Name}}Client - {{.Schema.Description}}{{end}}
type {{.Schema.Name}}Client struct {
	c *rapid.Client
}

{{if .Schema.Description}}// Dial{{.Schema.Name}} creates a new client for the {{.Schema.Name}} API.{{end}}
func Dial{{.Schema.Name}}(url string, protocol rapid.Protocol) (*{{.Schema.Name}}Client, error) {
	if protocol == nil {
		protocol = &rapid.DefaultProtocol{}
	}
	c, err := rapid.Dial(url, protocol)
	if err != nil {
		return nil, err
	}
	return &{{.Schema.Name}}Client{c}, nil
}

{{range .Schema.Routes}}
{{if .Description}}// {{.Name}} - {{.Description}}{{end}}
func (a *{{$.Schema.Name}}Client) {{.Name}}({{if .PathType}}{{.PathType|params}}, {{end}}{{if .RequestType}}req {{.RequestType|type}}, {{end}}{{if .QueryType}}query {{.QueryType|type}}{{end}}) ({{if .ResponseType}}{{if .StreamingResponse}}<-chan {{end}} {{.ResponseType|type}}, {{end}}{{if .StreamingResponse}}<-chan {{end}}error) {
{{if and (not .StreamingResponse) .ResponseType}}
	{{var "resp" .ResponseType}}
{{end}}

	{{if .StreamingResponse}}stream, err := a.c.DoStreaming({{else}}err := a.c.DoBasic({{end}}
		"{{.Method}}",
		{{ref "req" .RequestType}},
		{{if not .StreamingResponse}}{{ref "resp" .ResponseType}},{{end}}
		{{ref "query" .QueryType}},
		"{{.SimplifyPath}}",
		{{range .PathType|names}}
		{{.}},
		{{end}}
	)

{{if .StreamingResponse}}
	if err != nil {
		ec := make(chan error, 1)
		ec <- err
		return nil, ec
	}
	rc := make(chan {{.ResponseType|type}})
	ec := make(chan error)
	go func() (err error) {
		for {
			defer func() {
				recover()
				stream.Close()
			}()
			{{var "v" .ResponseType}}
			if err = stream.Next({{ref "v" .ResponseType}}); err != nil {
				ec <- err
				return err
			}
			rc <- v
		}
	}()
	return rc, ec
{{else}}
	{{if .ResponseType}}
	return resp, err
	{{else}}
	return err
	{{end}}
{{end}}
}
{{end}}

`
)

type goClientContext struct {
	Package     string
	Definitions map[string]string
	Schema      *Schema
}

func SchemaToGoClient(schema *Schema, pkg string, w io.Writer) error {
	sort.Sort(schema.Routes)
	// First, build map of type name to type definition
	definitions := map[string]string{}
	for _, t := range schema.Models() {
		name, definition := goTypeDefinition(t)
		if name != "" {
			definitions[name] = definition
		}
	}
	ctx := &goClientContext{
		Definitions: definitions,
		Package:     pkg,
		Schema:      schema,
	}
	goFuncs := template.FuncMap{
		"type":       goTypeReference,
		"title":      strings.Title,
		"params":     goPathTypeToParams,
		"names":      goPathNames,
		"var":        goTypeDecl,
		"ref":        goTypeRef,
		"needsalloc": func(t reflect.Type) bool { return t != nil && (t.Kind() == reflect.Ptr) },
		"isslice":    func(t reflect.Type) bool { return t != nil && t.Kind() == reflect.Slice },
	}
	tmpl := template.Must(template.New("go").Funcs(goFuncs).Parse(goTemplate))
	// return tmpl.Execute(w, ctx)

	gofmt := exec.Command("goimports")
	gofmtin, err := gofmt.StdinPipe()
	if err != nil {
		return err
	}
	defer gofmtin.Close()
	gofmtout, err := gofmt.StdoutPipe()
	if err != nil {
		return err
	}
	defer gofmtout.Close()
	if err := gofmt.Start(); err != nil {
		return err
	}
	go io.Copy(w, gofmtout)
	err = tmpl.Execute(gofmtin, ctx)
	if err != nil {
		gofmt.Process.Kill()
		return err
	}
	gofmtin.Close()
	return gofmt.Wait()
}
