package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/kingpin"

	"github.com/alecthomas/rapid/example"
)

var (
	createCommand = kingpin.Command("create", "Create a user.")
	createUser    = createCommand.Arg("username", "Username.").Required().String()

	listCommand = kingpin.Command("list", "List users.")
	listFilter  = listCommand.Arg("filter", "Glob-like filter matching usernames.").String()

	changesCommand = kingpin.Command("changes", "Show changes as they occur.")
)

func main() {
	command := kingpin.Parse()

	c, err := DialUsersClient("http://localhost:8090")
	kingpin.FatalIfError(err, "failed to dial server")

	switch command {
	case "create":
		err := c.CreateUser(&example.User{
			Name: *createUser,
		})
		kingpin.FatalIfError(err, "failed to create user")

	case "list":
		users, err := c.ListUsers(&example.UsersQuery{Name: *listFilter})
		kingpin.FatalIfError(err, "failed to retrieve users")
		for _, user := range users {
			fmt.Printf("%s (%d)\n", user.Name, user.ID)
		}

	case "changes":
		changes, err := c.Changes()
		kingpin.FatalIfError(err, "failed to retrieve changes")
		for {
			n, err := changes.Next()
			if err != nil {
				kingpin.CommandLine.Errorf(os.Stderr, "%s", err)
				changes.Close()
				return
			}
			fmt.Printf("%d\n", n)
			if n >= 5 {
				changes.Close()
				return
			}
		}
	}
}
