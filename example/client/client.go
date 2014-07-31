package main

import (
	"github.com/alecthomas/rapid"
)

type User struct {
	ID   int
	Name string
}

type UsersQuery struct {
	Name string `schema:"name"`
}

// UsersClient - An API for managing users.
type UsersClient struct {
	c *rapid.Client
}

// DialUsers creates a new client for the Users API.
func DialUsers(url string, protocol rapid.Protocol) (*UsersClient, error) {
	if protocol == nil {
		protocol = &rapid.DefaultProtocol{}
	}
	c, err := rapid.Dial(url, protocol)
	if err != nil {
		return nil, err
	}
	return &UsersClient{c}, nil
}

// CreateUser - Create a new user.
func (a *UsersClient) CreateUser(req *User) error {

	err := a.c.DoBasic(
		"POST",
		req,
		nil,
		nil,
		"/users",
	)

	return err

}

// ListUsers - Retrieve a list of known users.
func (a *UsersClient) ListUsers(query *UsersQuery) ([]*User, error) {

	resp := []*User{}

	err := a.c.DoBasic(
		"GET",
		nil,
		&resp,
		query,
		"/users",
	)

	return resp, err

}

// GetUser - Retrieve a single user by username.
func (a *UsersClient) GetUser(username string) (*User, error) {

	resp := &User{}

	err := a.c.DoBasic(
		"GET",
		nil,
		resp,
		nil,
		"/users/{username}",

		username,
	)

	return resp, err

}

// Changes - A streaming response of change IDs.
func (a *UsersClient) Changes() (<-chan int, <-chan error) {

	stream, err := a.c.DoStreaming(
		"GET",
		nil,

		nil,
		"/changes",
	)

	if err != nil {
		ec := make(chan error, 1)
		ec <- err
		return nil, ec
	}
	rc := make(chan int)
	ec := make(chan error)
	go func() (err error) {
		for {
			defer func() {
				recover()
				stream.Close()
			}()
			var v int
			if err = stream.Next(&v); err != nil {
				ec <- err
				return err
			}
			rc <- v
		}
	}()
	return rc, ec

}
