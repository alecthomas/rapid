package main

import (
	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/example"
)

// UsersClient - An API for managing users.
type UsersClient struct {
	C        rapid.Client
	Protocol rapid.Protocol
}

// DialUsersClient creates a new client for the Users API.
func DialUsers(protocol rapid.Protocol, url string) (*UsersClient, error) {
	c, err := rapid.Dial(protocol, url)
	if err != nil {
		return nil, err
	}
	return &UsersClient{C: c, Protocol: protocol}, nil
}

// NewUsersClient creates a new client for the Users API using an existing rapid.Client.
func NewUsersClient(protocol rapid.Protocol, client rapid.Client) *UsersClient {
	return &UsersClient{C: client, Protocol: protocol}
}

// CreateUser - Create a new user.
func (a *UsersClient) CreateUser(req *example.User) error {
	r := rapid.Request(a.Protocol, "POST", "/users").Body(req).Build()
	err := a.C.Do(r, nil)
	return err
}

// ListUsers - Retrieve a list of known users.
func (a *UsersClient) ListUsers(query *example.UsersQuery) ([]*example.User, error) {
	resp := []*example.User{}
	r := rapid.Request(a.Protocol, "GET", "/users").Query(query).Build()
	err := a.C.Do(r, &resp)
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
	r := rapid.Request(a.Protocol, "GET", "/users/changes").Build()
	stream, err := a.C.DoStreaming(r)
	return &ChangesStream{stream}, err
}

// GetUser - Retrieve a single user by username.
func (a *UsersClient) GetUser(username string) (*example.User, error) {
	resp := &example.User{}
	r := rapid.Request(a.Protocol, "GET", "/users/{username}", username).Build()
	err := a.C.Do(r, resp)
	return resp, err
}
