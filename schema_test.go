package rapid

import (
	"encoding/json"

	"github.com/stretchrcom/testify/assert"

	"testing"
)

var (
	schemaTestExpected = `
  {
      "$schema": "http://json-schema.org/draft-04/schema#",
      "title": "Product set",
      "type": "array",
      "items": {
          "title": "Product",
          "type": "object",
          "properties": {
              "id": {
                  "description": "The unique identifier for a product",
                  "type": "number"
              },
              "name": {
                  "type": "string"
              },
              "price": {
                  "type": "number"
              },
              "tags": {
                  "type": "array",
                  "items": {
                      "type": "string"
                  }
              },
              "dimensions": {
                  "type": "object",
                  "properties": {
                      "length": {"type": "number"},
                      "width": {"type": "number"},
                      "height": {"type": "number"}
                  },
                  "required": ["length", "width", "height"]
              },
              "warehouseLocation": {
                  "description": "Coordinates of the warehouse with the product",
                  "$ref": "http://json-schema.org/geo"
              }
          },
          "required": ["id", "name", "price"]
      }
  }
`
	schemaTestValue = `
  [
      {
          "id": 2,
          "name": "An ice sculpture",
          "price": 12.50,
          "tags": ["cold", "ice"],
          "dimensions": {
              "length": 7.0,
              "width": 12.0,
              "height": 9.5
          },
          "warehouseLocation": {
              "latitude": -78.75,
              "longitude": 20.4
          }
      },
      {
          "id": 3,
          "name": "A blue mouse",
          "price": 25.50,
          "dimensions": {
              "length": 3.1,
              "width": 1.0,
              "height": 1.0
          },
          "warehouseLocation": {
              "latitude": 54.4,
              "longitude": -32.7
          }
      }
  ]
  `
)

type schemaTestProduct struct {
	ID               int                   `json:"id" description:"The unique identifier for a product"`
	Name             string                `json:"name"`
	Price            float64               `json:"price"`
	Tags             []string              `json:"tags,omitempty"`
	Dimensions       *schemaTestDimensions `json:"dimensions,omitempty"`
	WarhouseLocation *schemaTestLocation   `json:"warehousLocation,omitempty" description:"Coordinates of the warehouse with the product"`
}

type schemaTestDimensions struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type schemaTestLocation struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type schemaTest []*schemaTestProduct

func TestSchemaSampleSchemaAndValue(t *testing.T) {
	value := schemaTest{}
	err := json.Unmarshal([]byte(schemaTestValue), &value)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(value))

	schema := &Schema{}
	err = json.Unmarshal([]byte(schemaTestExpected), schema)
	assert.NoError(t, err)
	assert.Equal(t, "array", schema.Type)
	assert.NotNil(t, schema.Items)
}

func TestSchemaFromArray(t *testing.T) {
	s := SchemaFrom("Product set", schemaTest{})
	json.Marshal(s)
}

func TestSchemaFromService(t *testing.T) {
	svc := Define("Test")
	svc.Route("Index").Get("/{id}").Response(&indexResponse{}).Streaming()
	svc.Route("Create").Post("/").Request(&indexRequest{})
	s := SchemaFromService(svc)
	json.Marshal(s)
}
