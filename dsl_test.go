package rapid

import (
	"github.com/stretchrcom/testify/assert"

	"testing"
)

func TestDefinitionRoute(t *testing.T) {
	d := Define("Test")
	d.Route("Something", "/some/path").Get()
	assert.NotNil(t, d.model.Resources["/some"])
	d.Route("Index", "/").Get()
	assert.NotNil(t, d.model.Resources["/"])
	assert.Equal(t, len(d.model.Resources["/"].Routes), 1)
	d.Route("Something Else", "/some").Get()
	assert.Equal(t, len(d.model.Resources["/"].Routes), 2)
}
