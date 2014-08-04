package main

import (
	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/example"
)

// UsersClient - An API for managing users.
type UsersClient struct {
	c rapid.Client
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

// NewUsers creates a new client for the Users API using an existing rapid.Client.
func NewUsers(client rapid.Client) *UsersClient {
	return &UsersClient{client}
}

// CreateUser - Create a new user.
func (a *UsersClient) CreateUser(req *example.User) error {
	r := rapid.Request("POST", "/users").Body(req).Build()
	err := a.c.Do(r, nil)
	return err
}

// ListUsers - Retrieve a list of known users.
func (a *UsersClient) ListUsers(query *example.UsersQuery) ([]*example.User, error) {
	resp := []*example.User{}
	r := rapid.Request("GET", "/users").Query(query).Build()
	err := a.c.Do(r, &resp)
	return resp, err
}

// GetUser - Retrieve a single user by username.
func (a *UsersClient) GetUser(username string) (*example.User, error) {
	resp := &example.User{}
	r := rapid.Request("GET", "/users/{username}", username).Build()
	err := a.c.Do(r, resp)
	return resp, err
}

type ChangesStream struct {
	stream rapid.ClientStream
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
	r := rapid.Request("GET", "/changes").Build()
	stream, err := a.c.DoStreaming(r)
	return &ChangesStream{stream}, err
}
