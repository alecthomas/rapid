package rapid

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchrcom/testify/assert"
)

type indexRequest struct {
	ID int
}

type indexResponse struct {
	ID int
}

type testServer struct {
	called bool
	id     int
	params Params
}

func (t *testServer) Index(params Params, req *indexRequest) (*indexResponse, error) {
	t.id = req.ID
	t.called = true
	t.params = params
	return &indexResponse{req.ID * 2}, Status(http.StatusOK)
}

func TestServerMethodDoesNotExist(t *testing.T) {
	svc := NewService("Test")
	svc.Route("Invalid").Get("/")
	_, err := NewServer(svc, &testServer{})
	assert.Error(t, err)
}

func TestServerMethodExists(t *testing.T) {
	svc := NewService("Test")
	svc.Route("Index").Get("/")
	_, err := NewServer(svc, &testServer{})
	assert.NoError(t, err)
}

func TestServerCallsMethod(t *testing.T) {
	svc := NewService("Test")
	svc.Route("Index").Get("/{id}").Request(&indexRequest{}).Response(&indexResponse{})

	test := &testServer{}
	svr, _ := NewServer(svc, test)

	rb := bytes.NewBuffer([]byte(`{"ID": 10}`))
	r, err := http.NewRequest("GET", "/hello", rb)
	assert.NoError(t, err)
	w := httptest.NewRecorder()
	svr.ServeHTTP(w, r)
	assert.True(t, test.called)
	assert.Equal(t, Params{"id": "hello"}, test.params)
	assert.Equal(t, 10, test.id)
	assert.Equal(t, 200, w.Code)
	assert.Equal(t, "{\"ID\":20}\n", w.Body.String())
}

type testChunkedServer struct {
	id int
}

func (t *testChunkedServer) Index(params map[string]interface{}) {

}

func TestServerChunkedResponses(t *testing.T) {
	svc := NewService("Test")
	svc.Route("Index").Get("/{id}").Response(&indexResponse{})
}
