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
	assert.Equal(t, w.String(), `{"routes":[{"request_type":null,"response_type":{"kind":"struct","name":"TestSchemaToJSONRequestType","fields":[{"kind":"map","name":"KV","key":{"kind":"string"},"value":{"kind":"string"},"annotation":"json:\"kv\""}]},"query_type":null,"path_type":null,"name":"Index","description":"","path":"/{id}","method":"GET","streaming_response":false,"success_status":0}],"name":"Test","description":""}`)
}
