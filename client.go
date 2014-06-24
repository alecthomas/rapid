package rapid

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"text/template"

	"io"
)

var (
	funcMap = template.FuncMap{
		"fixpointer": func(v string) string { return strings.TrimLeft(v, "*") },
	}

	clientTemplate = `package {{.Package}}

import (
  "encoding/json"

  "github.com/alecthomas/rapid"

  "{{.Import}}"
)
{{with .Service}}
type {{.Name}}Client struct {
  *rapid.Client
}

func Dial{{.Name}}(url string) (*{{.Name}}Client, error) {
  c, err := rapid.Dial(url)
  if err != nil {
    return nil, err
  }
  return &{{.Name}}Client{c}, nil
}

func (c *{{$.Service.Name}}Client) scalar(method, path string, req interface{}, rep interface{}) (int, error) {
  hrep, err := c.Client.Do(method, path, req, rep)
  err = json.NewDecoder(hrep.Body).Decode(rep)
  if err != nil {
    return -1, err
  }
  return hrep.StatusCode, err
}

{{range .Routes}}
{{if .StreamingResponse}}
func (c *{{$.Service.Name}}Client) {{.Name}}(req {{.RequestType}}) (chan {{.ResponseType}}, chan error) {
  ec := make(chan error, 1)
  rc := make(chan {{.ResponseType}})

  go func() {
    defer func() {
      close(ec)
      close(rc)
    }()

    hrep, err := c.Client.Do(method, path, req, rep)
    if err != nil {
      ec <- err
      return
    }
    r := http.NewChunkedReader(hrep.Body)
    dec := json.NewDecoder(r)

    for {
      p := &{{.ResponseType | fixpointer}}{}
      if err := dec.Decode(p); err != nil {
        ec <- err
        return
      }
    }
  }()

  return rc, ec
}
{{else}}
func (c *{{$.Service.Name}}Client) {{.Name}}(req {{.RequestType}}) ({{.ResponseType}}, error) {
  rep := &{{.ResponseType.String | fixpointer}}{}
  _, err := c.Client.Do("{{.HTTPMethod}}", "{{.Path}}", req, rep)
  return rep, err
}
{{end}}{{end}}{{end}}
`
)

func GenerateClient(pkg, imp string, service *Service, w io.Writer) error {
	t := template.Must(template.New(service.Name).Funcs(funcMap).Parse(clientTemplate))
	return t.Execute(w, struct {
		Package string
		Import  string
		Service *Service
	}{
		pkg,
		imp,
		service,
	})
}

type Client struct {
	url        string
	HTTPClient *http.Client
}

func (c *Client) Do(method, path string, req interface{}, rep interface{}) (*http.Response, error) {
	var reqb []byte
	var err error
	if req != nil {
		reqb, err = json.Marshal(req)
		if err != nil {
			return nil, err
		}
	}
	hreq, err := http.NewRequest(method, c.url+path, bytes.NewBuffer(reqb))
	if err != nil {
		return nil, err
	}
	return c.HTTPClient.Do(hreq)
}

func Dial(url string) (*Client, error) {
	return &Client{
		url:        url,
		HTTPClient: &http.Client{},
	}, nil
}
