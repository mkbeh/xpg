package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/mkbeh/postgres"
	"github.com/mkbeh/postgres/examples/sample/migrations"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var writer *postgres.Pool
var reader *postgres.Pool

var (
	host string
	port string
	user string
	pass string
	db   string
)

func init() {
	host = os.Getenv("POSTGRES_CLUSTER_HOST")
	port = os.Getenv("POSTGRES_CLUSTER_PORT")
	user = os.Getenv("POSTGRES_USER")
	pass = os.Getenv("POSTGRES_PASSWORD")
	db = os.Getenv("POSTGRES_DB")
}

func getUrlHandler(w http.ResponseWriter, req *http.Request) {
	type task struct {
		Id          int    `json:"id"`
		Description string `json:"description"`
	}

	rows, err := reader.Query(req.Context(), "SELECT id, description FROM tasks;")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	tasks := make([]task, 0)
	for rows.Next() {
		var v task
		if err := rows.Scan(&v.Id, &v.Description); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		tasks = append(tasks, v)
	}

	data, _ := json.Marshal(tasks)

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func postUrlHandler(w http.ResponseWriter, req *http.Request) {
	query := `INSERT INTO tasks VALUES (1, 'test1'), (2, 'test-2') ON CONFLICT DO NOTHING;`

	_, err := writer.Exec(req.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	} else {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("OK"))
	}
}

func main() {
	var err error

	cfg := &postgres.Config{
		ClusterHost:        host,
		ClusterPort:        port,
		ClusterReplicaPort: port,
		User:               user,
		Password:           pass,
		DB:                 db,
		MigrateEnabled:     true,
	}

	writer, err = postgres.NewWriter(
		postgres.WithConfig(cfg),
		postgres.WithClientID("test-client"),
		postgres.WithMigrations(migrations.FS),
	)
	if err != nil {
		log.Fatalln("failed init master pool", err)
	}
	defer writer.Close()

	reader, err = postgres.NewReader(
		postgres.WithConfig(cfg),
		postgres.WithClientID("test-client"),
	)
	if err != nil {
		log.Fatalln("failed init reader pool", err)
	}
	defer reader.Close()

	http.HandleFunc("/get", getUrlHandler)
	http.HandleFunc("/create", postUrlHandler)
	http.Handle("/metrics", promhttp.Handler())

	err = http.ListenAndServe("localhost:8080", nil)
	if err != nil {
		log.Fatalln("Unable to start web server:", err)
	}
}
