package schema

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

type Test struct {
	ID    int    `schema:"id"`
	Name  string `schema:"name"`
	Array []int  `schema:"array"`
}

func TestURLEncoder(t *testing.T) {
	s := &Test{
		ID:    10,
		Name:  "test",
		Array: []int{1, 2, 3},
	}
	v := EncodeStructToURLValues(s)
	assert.Equal(t, "array=1&array=2&array=3&id=10&name=test", v.Encode())
}
