package main

import (
	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/example"
)

// UsersClient - An API for managing users.
type UsersClient struct {
	C     rapid.Client
	Codec rapid.CodecFactory
}

// DialUsersClient creates a new client for the Users API.
func DialUsers(codec rapid.CodecFactory, url string) (*UsersClient, error) {
	c, err := rapid.Dial(codec, url)
	if err != nil {
		return nil, err
	}
	return &UsersClient{C: c, Codec: codec}, nil
}

// NewUsersClient creates a new client for the Users API using an existing rapid.Client.
func NewUsersClient(codec rapid.CodecFactory, client rapid.Client) *UsersClient {
	return &UsersClient{C: client, Codec: codec}
}

// CreateUser - Create a new user.
func (a *UsersClient) CreateUser(req *example.User) error {
	r := rapid.Request(a.Codec, "POST", "/users").Body(req).Build()
	err := a.C.Do(r, nil)
	return err
}

// ListUsers - Retrieve a list of known users.
func (a *UsersClient) ListUsers(query *example.UsersQuery) ([]*example.User, error) {
	resp := []*example.User{}
	r := rapid.Request(a.Codec, "GET", "/users").Query(query).Build()
	err := a.C.Do(r, &resp)
	return resp, err
}

// GetUser - Retrieve a single user by username.
func (a *UsersClient) GetUser(username string) (*example.User, error) {
	resp := &example.User{}
	r := rapid.Request(a.Codec, "GET", "/users/{username}", username).Build()
	err := a.C.Do(r, resp)
	return resp, err
}

// SetUserAvatar - Set user avatar.
func (a *UsersClient) SetUserAvatar(username string, req *rapid.FileUpload) (*example.User, error) {
	resp := &example.User{}
	r := rapid.Request(a.Codec, "POST", "/users/{username}/avatar", username).Body(req).Build()
	err := a.C.Do(r, resp)
	return resp, err
}
