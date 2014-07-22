package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/alecthomas/rapid"
)

type User struct {
	ID   int
	Name string
}

type UserService struct {
	// ...
}

func (u *UserService) ListUsers() ([]*User, error) {
	// Retrieve users
	users := []*User{&User{ID: 1, Name: "Bob"}, &User{ID: 2, Name: "Larry"}}
	return users, nil
}

func (u *UserService) CreateUser(user *User) (interface{}, error) {
	return nil, rapid.Status(403)
}

func (u *UserService) GetUser(params rapid.Params) (*User, error) {
	return nil, rapid.Status(403)
}

// Changes streams a sequence of integers to the client.
func (u *UserService) Changes() (chan int, chan error) {
	dc := make(chan int)
	ec := make(chan error)
	go func() {
		for i := 0; i < 10; i++ {
			dc <- i
			time.Sleep(time.Millisecond * 500)
		}
		close(dc)
	}()
	return dc, ec
}

func main() {
	users := rapid.NewService("Users")
	users.Route("ListUsers").Get("/users").Response([]*User{})
	users.Route("GetUser").Get("/users/{id}").Response(&User{})
	users.Route("CreateUser").Post("/users").Request(&User{})
	users.Route("Changes").Get("/changes").Streaming().Response(0)

	b, _ := json.Marshal(rapid.SchemaFromService(users))
	fmt.Printf("%s\n", b)

	service := &UserService{}
	server, err := rapid.NewServer(users, service)
	if err != nil {
		panic(err)
	}
	http.ListenAndServe(":8080", server)
}
