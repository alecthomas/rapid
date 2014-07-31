package main

import (
	"fmt"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/schema"
	"github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger("rapid/example/server")
)

type User struct {
	ID   int
	Name string
}

type UserService struct {
	users map[string]*User
}

type UserPath struct {
	Name string `schema:"username"`
}

type UsersQuery struct {
	Name string `schema:"name"`
}

func (u *UsersQuery) Fix() {
	if u.Name == "" {
		u.Name = "*"
	}
}

func (u *UserService) ListUsers(query *UsersQuery) ([]*User, error) {
	log.Info("ListUsers(%#v)", query)
	users := make([]*User, 0, len(u.users))
	query.Fix()

	for _, user := range u.users {
		match, _ := path.Match(query.Name, user.Name)
		if match {
			users = append(users, user)
		}
	}
	return users, nil
}

func (u *UserService) CreateUser(user *User) error {
	log.Info("CreateUser(%#v)", user)
	user.ID = len(u.users) + 1
	u.users[user.Name] = user
	return rapid.ErrorForStatus(http.StatusCreated)
}

func (u *UserService) GetUser(path *UserPath) (*User, error) {
	log.Info("GetUser(%s)", path.Name)
	user, ok := u.users[path.Name]
	if !ok {
		return nil, rapid.ErrorForStatus(http.StatusNotFound)
	}
	return user, nil
}

// Changes streams a sequence of integers to the client.
func (u *UserService) Changes(cancel rapid.CloseNotifierChannel) (chan int, chan error) {
	dc := make(chan int)
	ec := make(chan error)
	go func() {
		defer close(dc)
		for i := 0; i < 10; i++ {
			select {
			case dc <- i:
				time.Sleep(time.Millisecond * 500)
				err := rapid.Error(http.StatusGatewayTimeout, "timed out retrieving changes")
				log.Warning("Returning error %s", err)
				ec <- err
			case <-cancel:
				log.Warning("Cancelled")
				return
			}
		}
	}()
	return dc, ec
}

func main() {
	users := rapid.Define("Users").Description("An API for managing users.")
	users.Route("CreateUser").Post("/users").Request(&User{}).Description("Create a new user.")
	users.Route("ListUsers").Get("/users").Response([]*User{}).Description("Retrieve a list of known users.").Query(&UsersQuery{})
	users.Route("GetUser").Get("/users/{username}").Response(&User{}).Description("Retrieve a single user by username.").Path(&UserPath{})
	users.Route("Changes").Get("/changes").Streaming().Response(0).Description("A streaming response of change IDs.")

	// err := schema.SchemaToGoClient(users.Schema, "main", os.Stdout)
	w, _ := os.Create("./example/client/client.go")
	err := schema.SchemaToGoClient(users.Schema, "main", w)
	if err != nil {
		panic(err.Error())
	}
	w.Close()
	fmt.Println("Created new client code in ./example/client/client.go")

	service := &UserService{
		users: map[string]*User{},
	}
	server, err := rapid.NewServer(users, service)
	if err != nil {
		panic(err)
	}
	server.SetLogger(log)
	fmt.Println("Starting on http://0.0.0.0:8090")
	http.ListenAndServe(":8090", server)
}
