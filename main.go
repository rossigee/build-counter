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

func getNextID(db *sql.DB, name string) (int, error) {
	var nextID int
	query := "SELECT MAX(id) + 1 FROM builds WHERE name = $1"
	err := db.QueryRow(query, name).Scan(&nextID)
	if err != nil {
		return 0, err
	}
	return nextID, nil
}

func handler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Query().Get("name")
		if name == "" {
			http.Error(w, "Missing name parameter", http.StatusBadRequest)
			return
		}

		nextID, err := getNextID(db, name)
		if err != nil {
			log.Printf("Error fetching next ID for name %s: %v", name, err) // Log the error
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

func addBuildHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method is not supported.", http.StatusMethodNotAllowed)
			return
		}

		var b struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err := db.Exec("INSERT INTO builds (name) VALUES ($1)", b.Name)
		if err != nil {
			log.Printf("Error inserting new build record: %v", err)
			http.Error(w, "Error inserting new record", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func main() {
	// Use os.Getenv to read the environment variable for your connection string
	connStr := os.Getenv("DB_CONNECTION_STRING")
	if connStr == "" {
		log.Fatal("DB_CONNECTION_STRING environment variable is not set")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	http.HandleFunc("/nextid", handler(db))
	http.HandleFunc("/addbuild", addBuildHandler(db))

	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
