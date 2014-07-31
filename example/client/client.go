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

type ChangesStream struct {
	stream *rapid.ClientStream
}

func (s *ChangesStream) Next() (int, error) {
	var v int
	err := s.stream.Next(&v)
	return v, err
}

func (s *ChangesStream) Close() error {
	return s.stream.Close()
}

// Changes - A streaming response of change IDs.
func (a *UsersClient) Changes() (*ChangesStream, error) {

	stream, err := a.c.DoStreaming(
		"GET",
		nil,

		nil,
		"/changes",
	)

	return &ChangesStream{stream}, err

}
