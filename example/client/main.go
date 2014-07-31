package main

import (
	"fmt"

	"github.com/alecthomas/kingpin"
)

var (
	createCommand = kingpin.Command("create", "Create a user.")
	createUser    = createCommand.Arg("username", "Username.").Required().String()

	listCommand = kingpin.Command("list", "List users.")
	listFilter  = listCommand.Arg("filter", "Glob-like filter matching usernames.").String()
)

func main() {
	command := kingpin.Parse()

	c, err := DialUsers("http://localhost:8090", nil)
	kingpin.FatalIfError(err, "failed to dial server")

	switch command {
	case "create":
		err := c.CreateUser(&User{
			Name: *createUser,
		})
		kingpin.FatalIfError(err, "failed to create user")

	case "list":
		users, err := c.ListUsers(&UsersQuery{Name: *listFilter})
		kingpin.FatalIfError(err, "failed to retrieve users")
		for _, user := range users {
			fmt.Printf("%s (%d)\n", user.Name, user.ID)
		}
	}
}
