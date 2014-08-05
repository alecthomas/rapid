package schema_test

import (
	"bytes"
	"testing"

	"github.com/stretchrcom/testify/assert"

	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/schema"
)

type TestSchemaToJSONRequestType struct {
	KV map[string]string `json:"kv"`
}

func TestSchemaToJSON(t *testing.T) {
	s := rapid.Define("Test")
	s.Route("Index").Get("/{id}").Response(&TestSchemaToJSONRequestType{})
	w := bytes.NewBuffer(nil)
	assert.NoError(t, schema.SchemaToJSON(s.Schema, w))
	println(w.String())
}
