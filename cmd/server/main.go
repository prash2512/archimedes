package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/prashanth/archimedes/internal/blocks"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "archimedes")
	})

	mux.HandleFunc("GET /api/blocks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blocks.Catalog)
	})

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
