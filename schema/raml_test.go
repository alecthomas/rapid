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
	d.Route("List").Get("/user").Response(200, []TestRAMLResponseType{})
	d.Route("Get").Get("/user/{id}").Query(TestRAMLQueryType{}).Path(TestRAMLPathType{}).Response(200, TestRAMLResponseType{})
	return d.Schema
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
	fmt.Printf("EXAMPLE = %s\n", makeRAMLExample(reflect.TypeOf(TestRAMLNestedStruct{})))
}
