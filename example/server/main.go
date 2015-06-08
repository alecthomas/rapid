package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/alecthomas/go-logging"
	"github.com/alecthomas/kingpin"

	"github.com/alecthomas/rapid"
	"github.com/alecthomas/rapid/example"
)

var (
	log = logging.MustGetLogger("rapid/example/server")
)

func main() {
	kingpin.Parse()

	users := example.UserServiceDefinition()
	// err := rapid.SchemaToGoClient(users.Schema, "main", os.Stdout)
	w, _ := os.Create("./example/client/client.go")
	err := rapid.SchemaToGoClient(users, false, "main", w)
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
	kingpin.FatalIfError(http.ListenAndServe(":8090", server), "failed to start server")
}
