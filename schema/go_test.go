package schema

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/alecthomas/rapid/schema"

	"github.com/alecthomas/rapid"
)

type TestGoUser struct {
	Name string
}

type TestGoQuery struct {
	Name string
}

func TestGo(t *testing.T) {
	w := &bytes.Buffer{}
	d := rapid.Define("Test")
	users := d.Resource("Users", "/users")
	users.Route("List", "/users").Get().Query(&TestGoQuery{}).Response(200, []*TestGoUser{})
	users.Route("Get", "/users/{id}").Get().Response(200, &TestGoUser{})
	err := schema.SchemaToGoClient(d.Build(), false, "main", w)
	assert.NoError(t, err)
	fmt.Printf("%s\n", w.String())
}
