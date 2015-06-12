# RESTful API Daemons (and Clients) for Go

This Go package provides facilities for building server-side RESTful APIs. An
API is defined via a DSL in Go. This definition can be used to generate
[RAML](http://raml.org) schemas and nicely idiomatic Go client code.

## Example

Here's an example of defining a basic user service:

```go
type User struct {
  ID int
  Name string
}

type IDPath struct {
  ID string `schema:"id"`
}

users := rapid.Define("Users")
users.Route("ListUsers").Get("/users").Response(http.StatusOK, []*User{})
users.Route("GetUser").Get("/users/{id}").Path(&IDPath{}).Response(http.StatusOK, &User{})
users.Route("CreateUser").Post("/users").Request(http.StatusCreated, &User{})
```

Once your schema is defined you can create a service implementation. Each
route maps to a method on a service struct:

```go
type UserService struct {
  // ...
}

func (u *UserService) ListUsers() ([]*User, error) {
  // Retrieve users
  users := []*User{&User{ID: 1, Name: "Bob"}, &User{ID: 2, Name: "Larry"}}
  return users, nil
}

func (u *UserService) CreateUser(user *User) error {
  return rapid.ErrorForStatus(403)
}

func (u *UserService) GetUser(path *IDPath) (*User, error) {
  return nil, rapid.ErrorForStatus(403)
}
```

Finally, bind the service definition to the implementation:

```go
service := &UserService{}
server, err := rapid.NewServer(users, service)
http.ListenAndServe(":8080", server)
```

## Encoding

The encoding, headers, etc. that different REST protocols use differs
considerably. To cater for this, RAPID supports four interfaces for
encoding/decoding requests and  responses:

```go
// Encoding and decoding requests on the client and server, respectively.
type RequestCodec interface {
  // Encode request on client.
  EncodeRequest() (headers http.Header, body io.ReadCloser, err error)
  // Decode request.
  DecodeRequest(r *http.Request) error
}

// Encoding and decoding responses on the server and client, respectively.
type ResponseCodec interface {
  // Encode response into w. http.Request is included to allow Accept-based
  // responses.
  EncodeResponse(r *http.Request, w http.ResponseWriter, status int, err error) error
  // Decode response from r.
  DecodeResponse(r *http.Response) error
}

type Codec interface {
  RequestCodec
  ResponseCodec
}

type CodecFactory func(v interface{}) Codec
```

The included implementation of `Codec` and
`CodecFactory` supports a JSON-based API. This can be
completely replaced by your own implementation (eg. encoding using Protocol
Buffers, Avro, Thrift, etc.).

Additionally, individual types used in the definition of responses and
requests can implement these interfaces to override the default codec. This
can be seen in the included `rapid.FileDownload`, `rapid.Upload` and
`rapid.RawBytes` types. `rapid.FileDownload`, for example, sets the
appropriate `Content-Type` and `Content-Disposition: attachment` headers.
