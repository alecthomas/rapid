package rapid

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type TestGoUser struct {
	Name string
}

type TestGoQuery struct {
	Name string
}

func TestGo(t *testing.T) {
	w := &bytes.Buffer{}
	d := Define("Test")
	users := d.Resource("Users", "/users")
	users.Route("List", "/users").Get().Query(&TestGoQuery{}).Response(200, []*TestGoUser{})
	users.Route("Get", "/users/{id}").Get().Response(200, &TestGoUser{})
	err := SchemaToGoClient(d.Build(), false, "main", w)
	assert.NoError(t, err)
	fmt.Printf("%s\n", w.String())
}
