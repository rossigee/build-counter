package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	_ "github.com/lib/pq"
)

type Response struct {
	NextID int `json:"next_id"`
}

func startBuildHandler() http.HandlerFunc {
	log.Println("Initialising 'startBuildHandler' function...")

	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
			return
		}

		build_id := r.URL.Query().Get("build_id")
		if build_id == "" {
			http.Error(w, "Missing 'build_id' parameter", http.StatusBadRequest)
			return
		}

		var nextID int
		query := "INSERT INTO builds (name, build_id, started) VALUES ($1, $2, now()) RETURNING id;"
		db, err := connectDatabase()
		if err != nil {
			log.Printf("Unable to connect to database: %v", err)
			http.Error(w, "Error fetching next ID", http.StatusInternalServerError)
			return
		}
		defer db.Close()
		err = db.QueryRow(query, name, build_id).Scan(&nextID)
		if err != nil {
			log.Printf("Error inserting new build record: %v", err)
			http.Error(w, "Error fetching next ID", http.StatusInternalServerError)
			return
		}

		resp := Response{NextID: nextID}
		jsonResp, err := json.Marshal(resp)
		if err != nil {
			log.Printf("Error marshaling JSON response: %v", err) // Log this error as well
			http.Error(w, "Error formatting response", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(jsonResp)
	}
}

func finishBuildHandler() http.HandlerFunc {
	log.Println("Initialising 'finishBuildHandler' function...")

	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "Missing 'name' parameter", http.StatusBadRequest)
			return
		}

		build_id := r.URL.Query().Get("build_id")
		if build_id == "" {
			http.Error(w, "Missing 'build_id' parameter", http.StatusBadRequest)
			return
		}

		query := "UPDATE builds SET finished = NOW() WHERE name = $1 AND build_id = $2"
		db, err := connectDatabase()
		if err != nil {
			log.Printf("Unable to connect to database: %v", err)
			http.Error(w, "Error updating finish time", http.StatusInternalServerError)
			return
		}
		defer db.Close()
		_, err = db.Exec(query, name, build_id)
		if err != nil {
			log.Printf("Error updating finish time for name %s: %v", name, err)
			http.Error(w, "Error updating finish time", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func connectDatabase() (*sql.DB, error) {
	// Use os.Getenv to read the environment variable for your connection string
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}

	return db, nil
}

func main() {
	http.HandleFunc("/start", startBuildHandler())
	http.HandleFunc("/finish", finishBuildHandler())

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
