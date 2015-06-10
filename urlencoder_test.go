package rapid

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type Test struct {
	ID       int           `schema:"id"`
	Name     string        `schema:"name"`
	Array    []int         `schema:"array"`
	Duration time.Duration `schema:"duration"`
}

func TestURLEncoder(t *testing.T) {
	s := &Test{
		ID:       10,
		Name:     "test",
		Array:    []int{1, 2, 3},
		Duration: time.Hour,
	}
	v := EncodeStructToURLValues(s)
	assert.Equal(t, "array=1&array=2&array=3&duration=1h0m0s&id=10&name=test", v.Encode())
}
