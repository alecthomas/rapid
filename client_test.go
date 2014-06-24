package rapid

import (
	"bytes"
	"testing"

	"github.com/stretchrcom/testify/assert"
)

func TestClient(t *testing.T) {
	svc := NewService("Test")
	svc.Route("Index").Get("/").Request(&indexRequest{}).Response(&indexResponse{})
	buf := bytes.NewBuffer(nil)
	err := GenerateClient("test", "github.com/alecthomas/rapid", svc, buf)
	assert.NoError(t, err)
}
