package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/alecthomas/rapid/example"

	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/schema"
	"github.com/op/go-logging"
)

var (
	log = logging.MustGetLogger("rapid/example/server")
)

func main() {
	users := example.UserServiceDefinition()
	// err := schema.SchemaToGoClient(users.Schema, "main", os.Stdout)
	w, _ := os.Create("./example/client/client.go")
	err := schema.SchemaToGoClient(users.Schema, "main", w)
	if err != nil {
		panic(err.Error())
	}
	w.Close()
	fmt.Println("Created new client code in ./example/client/client.go")

	service := example.NewUserService()
	server, err := rapid.NewServer(users, service)
	if err != nil {
		panic(err)
	}
	server.SetLogger(log)
	fmt.Println("Starting on http://0.0.0.0:8090")
	http.ListenAndServe(":8090", server)
}
