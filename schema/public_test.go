package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchrcom/testify/assert"

	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/schema"
)

type TestSchemaToPublicRequestType struct {
	KV map[string]string `json:"kv"`
}

func TestSchemaToJSON(t *testing.T) {
	s := rapid.Define("Test")
	s.Route("Index").Get("/{id}").Response(&TestSchemaToPublicRequestType{})
	public := schema.SchemaToPublic(s.Schema)
	b, _ := json.Marshal(public)
	// fmt.Printf("%s\n", b)
	assert.Equal(t, string(b), `{"routes":[{"response_type":{"type":"TestSchemaToPublicRequestType"},"name":"Index","path":"/{id}","method":"GET"}],"name":"Test","Types":{"TestSchemaToPublicRequestType":{"type":"TestSchemaToPublicRequestType","fields":[{"type":"map","name":"kv","key":{"type":"string"},"value":{"type":"string"}}]}}}`)
}
