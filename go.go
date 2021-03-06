package rapid

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/alecthomas/template"
)

var (
	goTemplate = `package {{.Package}}

import (
{{range $key, $value := .Imports}}
	"{{$key}}"{{end}}
)

{{if .Schema.Description}}// {{.Schema.Name|visibility}}Client - {{.Schema.Description}}{{end}}
type {{.Schema.Name|visibility}}Client struct {
	C rapid.Client
	Codec rapid.CodecFactory
}

{{if .Schema.Description}}// {{"Dial"|visibility}}{{.Schema.Name}}Client creates a new client for the {{.Schema.Name}} API.{{end}}
func {{"Dial"|visibility}}{{.Schema.Name}}(codec rapid.CodecFactory, url string) (*{{.Schema.Name|visibility}}Client, error) {
	c, err := rapid.Dial(codec, url)
	if err != nil {
		return nil, err
	}
	return &{{.Schema.Name|visibility}}Client{C: c, Codec: codec}, nil
}


{{if .Schema.Description}}// {{"New"|visibility}}{{.Schema.Name}}Client creates a new client for the {{.Schema.Name}} API using an existing rapid.Client.{{end}}
func {{"New"|visibility}}{{.Schema.Name}}Client(codec rapid.CodecFactory, client rapid.Client) *{{.Schema.Name|visibility}}Client {
	return &{{.Schema.Name|visibility}}Client{C: client, Codec: codec}
}

{{range .Schema.Resources}}
{{range .Routes}}
{{$response := .DefaultResponse}}
{{if not .Hidden}}
{{if $response.Streaming}}
type {{.Name|visibility}}Stream struct {
	stream rapid.ClientStream
}

func (s *{{.Name|visibility}}Stream) Next() ({{$response.Type|type}}, error) {
	{{var "v" $response.Type}}
	err := s.stream.Next({{ref "v" $response.Type}})
	return v, err
}

func (s *{{.Name|visibility}}Stream) Close() error {
	return s.stream.Close()
}
{{end}}
{{if .Description}}// {{.Name}} - {{.Description}}{{end}}
func (a *{{$.Schema.Name|visibility}}Client) {{.Name}}({{if .PathType}}{{.PathType|params}}, {{end}}{{if .RequestType}}req {{.RequestType|type}}, {{end}}{{if .QueryType}}query {{.QueryType|type}}, {{end}}) ({{if $response.Streaming}}*{{.Name|visibility}}Stream, {{else}}{{if $response.Type}}{{$response.Type|type}}, {{end}}{{end}}error) {
	{{if and (not $response.Streaming) $response.Type}}\
	{{var "resp" $response.Type}}
	{{end}}\
	r := rapid.Request(a.Codec, "{{.Method}}", "{{.SimplifyPath}}", {{range .PathType|names}}{{.}},{{end}}){{if .QueryType}}.Query(query){{end}}{{if .RequestType}}.Body(req){{end}}.Build()
	{{if $response.Streaming}}stream, err := a.C.DoStreaming({{else}}err := a.C.Do({{end}}r, {{if not $response.Streaming}}{{ref "resp" $response.Type}},{{end}})
	{{if $response.Streaming}}return &{{.Name|visibility}}Stream{stream}, err{{else}}{{if $response.Type}}return resp, err{{else}}return err{{end}}{{end}}
}
{{end}}
{{end}}
{{end}}

`
)

func goTypeReference(pkg string, t reflect.Type) string {
	// Named types.
	if t.Name() != "" {
		if t.PkgPath() == pkg {
			return t.Name()
		}
		return fmt.Sprintf("%s", t)
	}

	switch t.Kind() {
	case reflect.Ptr:
		return "*" + goTypeReference(pkg, t.Elem())

	case reflect.Interface:
		return "interface{}"

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
		return "[]" + goTypeReference(pkg, t.Elem())

	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", goTypeReference(pkg, t.Key()), goTypeReference(pkg, t.Elem()))
	}
	panic(fmt.Sprintf("unsupported type %s", t))
}

func goTypeDefinition(pkg string, t reflect.Type) (name string, definition string) {
	switch t.Kind() {
	case reflect.Struct:
		out := &bytes.Buffer{}
		out.WriteString("struct {\n")
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			fmt.Fprintf(out, "\t%s %s", f.Name, goTypeReference(pkg, f.Type))
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
		return goTypeDefinition(pkg, t.Elem())

	default:
		return "", goTypeReference(pkg, t)
	}
}

func goPathTypeToParams(pkg string, t reflect.Type) string {
	switch t.Kind() {
	case reflect.Struct:
		out := []string{}
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			name := f.Tag.Get("schema")
			if name == "" {
				name = f.Name
			}
			out = append(out, fmt.Sprintf("%s %s", name, goTypeReference(pkg, f.Type)))
		}
		return strings.Join(out, ", ")

	case reflect.Ptr:
		return goPathTypeToParams(pkg, t.Elem())
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

func goTypeDecl(pkg string, name string, t reflect.Type) string {
	switch t.Kind() {
	case reflect.Slice:
		if t.Name() != "" {
			return fmt.Sprintf("%s := %s{}", name, goTypeReference(pkg, t))
		}
		return fmt.Sprintf("%s := []%s{}", name, goTypeReference(pkg, t.Elem()))

	case reflect.Struct:
		typ := fmt.Sprintf("%s", t)
		if pkg == t.PkgPath() {
			typ = t.Name()
		}
		return fmt.Sprintf("%s := &%s{}", name, typ)

	case reflect.Ptr:
		return goTypeDecl(pkg, name, t.Elem())

	default:
		return fmt.Sprintf("var %s %s", name, goTypeReference(pkg, t))
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

type goClientContext struct {
	Imports map[string]struct{}
	Package string
	Schema  *Schema
	Private bool
}

func SchemaToGoClient(schema *Schema, private bool, pkg string, w io.Writer) error {
	imports := map[string]struct{}{
		"github.com/alecthomas/rapid": struct{}{},
	}
	for _, t := range schema.Types() {
		if t.PkgPath() == "" || t.PkgPath() == pkg {
			continue
		}
		imports[t.PkgPath()] = struct{}{}
	}
	ctx := &goClientContext{
		Imports: imports,
		Package: filepath.Base(pkg),
		Schema:  schema,
		Private: private,
	}
	goFuncs := template.FuncMap{
		"type":        func(t reflect.Type) string { return goTypeReference(pkg, t) },
		"title":       strings.Title,
		"params":      func(t reflect.Type) string { return goPathTypeToParams(pkg, t) },
		"names":       goPathNames,
		"var":         func(name string, t reflect.Type) string { return goTypeDecl(pkg, name, t) },
		"ref":         goTypeRef,
		"needsalloc":  func(t reflect.Type) bool { return t != nil && (t.Kind() == reflect.Ptr) },
		"isslice":     func(t reflect.Type) bool { return t != nil && t.Kind() == reflect.Slice },
		"isencodable": func(v interface{}) bool { _, ok := v.(RequestCodec); return ok },
		"visibility": func(name string) string {
			if private {
				return lowerFirst(name)
			}
			return name
		},
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

func lowerFirst(s string) string {
	if s == "" {
		return ""
	}
	r, n := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[n:]
}
