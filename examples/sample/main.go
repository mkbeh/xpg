package main

import (
	"log"
	"net/http"

	"github.com/mkbeh/postgres"
)

func getUrlHandler(w http.ResponseWriter, req *http.Request) {
	// todo
}

func putUrlHandler(w http.ResponseWriter, req *http.Request) {
	// todo
}

func urlHandler(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		getUrlHandler(w, req)

	case "PUT":
		putUrlHandler(w, req)

	default:
		w.Header().Add("Allow", "GET, PUT")
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func main() {
	cfg := &postgres.Config{
		ClusterHost:        "127.0.0.1",
		ClusterPort:        "5432",
		ClusterReplicaPort: "5432",
		User:               "cyberpunk",
		Password:           "cyberpunk",
		DB:                 "postgres",
	}

	writer, err := postgres.NewWriter(
		postgres.WithConfig(cfg),
		postgres.WithClientID("test-client"),
	)
	if err != nil {
		panic(err)
	}
	defer writer.Close()

	err = http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatalln("Unable to start web server:", err)
	}
}
