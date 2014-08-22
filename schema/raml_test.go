package schema

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchrcom/testify/assert"

	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/schema"
)

type ID struct {
	ID string
}

type TestRAMLQueryType struct {
	Age int `schema:"age"`
}

type TestRAMLPathType struct {
	ID int `schema:"id"`
}

type TestRAMLResponseType struct {
	Name string
}

func makeTestSchema() *schema.Schema {
	d := rapid.Define("Test")
	d.Route("List", "/user").Get().Response(200, []TestRAMLResponseType{})
	d.Route("Get", "/user/{id}").Get().Query(TestRAMLQueryType{}).Path(TestRAMLPathType{}).Response(200, TestRAMLResponseType{})
	return d.Build()
}

func TestSchemaToRAML(t *testing.T) {
	w := &bytes.Buffer{}
	err := schema.SchemaToRAML("http://localhost:8080", makeTestSchema(), w)
	assert.NoError(t, err)
	fmt.Printf("%s\n", w.String())
}

type TestRAMLNestedStruct struct {
	PointerType         *TestRAMLPathType
	NonPointerQueryType TestRAMLQueryType
	SliceOfPointers     []*TestRAMLQueryType
	SliceOfNonPointers  []TestRAMLQueryType
}

func TestMakeExample(t *testing.T) {
	raml := makeRAMLExample(reflect.TypeOf(TestRAMLNestedStruct{}), true)
	assert.Equal(t, `{
  "PointerType": {
    "ID": 0
  },
  "NonPointerQueryType": {
    "Age": 0
  },
  "SliceOfPointers": [
    {
      "Age": 0
    }
  ],
  "SliceOfNonPointers": [
    {
      "Age": 0
    }
  ]
}`, raml)
}
