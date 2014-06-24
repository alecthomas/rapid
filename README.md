# RESTful API Daemons (and Clients) for Go

This Go package provides facilities for building server-side RESTful APIs. An
API schema is defined via a DSL in Go.

Here's an example of defining a basic user service:

```go
type User struct {
  ID int
  Name string
}

schema := rapid.NewService("Users")
schema.Route("ListUsers").Get("/users").Response([]*User{})
schema.Route("CreateUser").Post("/users").Request(&User{})
schema.Route("GetUser").Get("/users/{id}").Response(&User{})
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
  return rapid.Status(403, "can't create users")
}

func (u *UserService) GetUser(user *User) (*User, error) {
  return nil, rapid.Status(403, "can't retrieve user")
}
```

Finally, bind the service definition to the implementation:

```go
service := &UserService{}
http.Handle("/users", rapid.NewServer(schema, service))
```
