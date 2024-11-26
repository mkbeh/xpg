package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/mkbeh/postgres"
	"github.com/mkbeh/postgres/examples/sample/migrations"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func getUrlHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Println("test get")
}

func putUrlHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Println("test put")
}

func main() {
	cfg := &postgres.Config{
		ClusterHost:        "127.0.0.1",
		ClusterPort:        "5432",
		ClusterReplicaPort: "5432",
		User:               "cyberpunk",
		Password:           "cyberpunk",
		DB:                 "postgres",
		MigrateEnabled:     true,
	}

	writer, err := postgres.NewWriter(
		postgres.WithConfig(cfg),
		postgres.WithClientID("test-client"),
		postgres.WithMigrations(migrations.FS),
	)
	if err != nil {
		log.Fatalln("failed init master pool", err)
	}
	defer writer.Close()

	reader, err := postgres.NewReader(
		postgres.WithConfig(cfg),
		postgres.WithClientID("test-client"),
	)
	if err != nil {
		log.Fatalln("failed init reader pool", err)
	}
	defer reader.Close()

	http.HandleFunc("/get", getUrlHandler)
	http.HandleFunc("/put", putUrlHandler)
	http.Handle("/metrics", promhttp.Handler())

	err = http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatalln("Unable to start web server:", err)
	}
}
