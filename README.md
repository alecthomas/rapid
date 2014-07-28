# RESTful API Daemons (and Clients) for Go

This Go package provides facilities for building server-side RESTful APIs. An
API is defined via a DSL in Go. This definition can be used to generate
[JSON Hyper-Schema](http://json-schema.org) schemas which can in turn be used
to generate elegant Go client code for the API.

## Example

Here's an example of defining a basic user service:

```go
type User struct {
  ID int
  Name string
}

users := rapid.Define("Users")
users.Route("ListUsers").Get("/users").Response([]*User{})
users.Route("GetUser").Get("/users/{id}").Response(&User{})
users.Route("CreateUser").Post("/users").Request(&User{})
users.Route("Changes").Get("/changes").Streaming().Response(0)
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

func (u *UserService) GetUser(params rapid.Params) (*User, error) {
  return nil, rapid.ErrorForStatus(403)
}

// Changes streams a sequence of integers to the client.
func (u *UserService) Changes(closeNotifier rapid.CloseNotifierChannel) (chan int, chan error) {
  dc := make(chan int)
  ec := make(chan error)
  go func() {
    for i := 0; i < 10; i++ {
      select {
      case dc <- i:
        time.Sleep(time.Millisecond * 500)
      case <-closeNotifier:
        return
      }
    }
    close(dc)
  }()
  return dc, ec
}
```

Finally, bind the service definition to the implementation:

```go
service := &UserService{}
server, err := rapid.NewServer(users, service)
http.ListenAndServe(":8080", server)
```


## JSON Hyper-Schema

Rapid can generate [JSON Hyper-Schema](http://json-schema.org) schemas from
your service definitions by calling `rapid.SchemaFromService(service)`.
Additionally, the `rapid` command-line utility can then be used to generate Go
client code from this schema.

The above service will generate the following schema:

```json
{
  "title": "Users",
  "links": [
    {
      "rel": "ListUsers",
      "href": "/users",
      "method": "GET",
      "targetSchema": {
        "type": "array",
        "items": {
          "required": [
            "ID",
            "Name"
          ],
          "type": "object",
          "properties": {
            "ID": {
              "type": "number"
            },
            "Name": {
              "type": "string"
            }
          }
        }
      }
    },
    {
      "rel": "GetUser",
      "href": "/users/{id}",
      "method": "GET",
      "targetSchema": {
        "required": [
          "ID",
          "Name"
        ],
        "type": "object",
        "properties": {
          "ID": {
            "type": "number"
          },
          "Name": {
            "type": "string"
          }
        }
      }
    },
    {
      "rel": "CreateUser",
      "href": "/users",
      "method": "POST",
      "schema": {
        "required": [
          "ID",
          "Name"
        ],
        "type": "object",
        "properties": {
          "ID": {
            "type": "number"
          },
          "Name": {
            "type": "string"
          }
        }
      }
    },
    {
      "rel": "Changes",
      "href": "/changes",
      "method": "GET",
      "transferEncoding": "chunked",
      "targetSchema": {
        "type": "number"
      }
    }
  ]
}
```
