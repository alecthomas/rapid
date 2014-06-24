package main

import (
	"errors"
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
	return nil, rapid.StatusMessage(403, "can't create users")
}

func (u *UserService) GetUser() (*User, error) {
	return nil, rapid.StatusMessage(403, "can't retrieve user")
}

func (u *UserService) Changes() (chan int, chan error) {
	dc := make(chan int)
	ec := make(chan error)
	go func() {
		for i := 0; i < 10; i++ {
			dc <- i
			time.Sleep(time.Millisecond * 500)
		}
		ec <- errors.New("BAD")
	}()
	return dc, ec
}

func main() {
	schema := rapid.NewService("Users")
	schema.Route("ListUsers").Get("/users").Response([]*User{})
	schema.Route("GetUser").Get("/users/{id}").Response(&User{})
	schema.Route("CreateUser").Post("/users").Request(&User{})
	schema.Route("Changes").Get("/changes").Streaming().Response(0)

	service := &UserService{}
	server, err := rapid.NewServer(schema, service)
	if err != nil {
		panic(err)
	}
	http.ListenAndServe(":8080", server)
}
