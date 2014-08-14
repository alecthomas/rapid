package rapid

import (
	"github.com/stretchrcom/testify/assert"

	"testing"
)

func TestDefinitionRoute(t *testing.T) {
	d := Define("Test")
	d.Route("Something Else", "/some").Get()
	assert.NotNil(t, d.model.ResourceByPath("/some"))
	assert.Equal(t, 1, len(d.model.ResourceByPath("/some").Routes))
	d.Route("Something", "/some/path").Get()
	assert.Equal(t, 2, len(d.model.ResourceByPath("/some").Routes))
	d.Route("Index", "/").Get()
	assert.NotNil(t, d.model.ResourceByPath("/"))
	assert.Equal(t, 1, len(d.model.ResourceByPath("/").Routes))
}

func TestDefineDSLViaResources(t *testing.T) {
	d := Define("Test")
	some := d.Resource("Some", "/some")
	some.Route("ListSome", "/some").Get()
	some.Route("DeleteSome", "/some/{id}").Delete()
	assert.Equal(t, 2, len(d.model.ResourceByPath("/some").Routes))
}
