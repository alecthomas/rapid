package example

import (
	"net/http"
	"path"
	"time"

	"github.com/alecthomas/rapid"

	"github.com/alecthomas/go-logging"
)

var (
	log = logging.MustGetLogger("example")
)

type User struct {
	ID   int
	Name string
}

func UserServiceDefinition() *rapid.Definition {
	api := rapid.Define("Users").Description("An API for managing users.")
	api.Route("CreateUser").Post("/").Request(&User{}).Description("Create a new user.")
	api.Route("ListUsers").Get("/").Response(200, []*User{}).Description("Retrieve a list of known users.").Query(&UsersQuery{})
	api.Route("GetUser").Get("/{username}").Response(200, &User{}).Description("Retrieve a single user by username.").Path(&UserPath{})
	api.Route("Changes").Get("/changes").Streaming().Response(200, 0).Description("A streaming response of change IDs.")
	return api
}

type UserService struct {
	users map[string]*User
}

func NewUserService() *UserService {
	return &UserService{
		users: make(map[string]*User),
	}
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
	log.Infof("ListUsers(%#v)", query)
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
	log.Infof("CreateUser(%#v)", user)
	user.ID = len(u.users) + 1
	u.users[user.Name] = user
	return rapid.ErrorForStatus(http.StatusCreated)
}

func (u *UserService) GetUser(path *UserPath) (*User, error) {
	log.Infof("GetUser(%s)", path.Name)
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
				log.Warningf("Returning error %s", err)
				ec <- err
			case <-cancel:
				log.Warningf("Cancelled")
				return
			}
		}
	}()
	return dc, ec
}
