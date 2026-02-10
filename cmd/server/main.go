package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/prashanth/archimedes/internal/blocks"
)

func main() {
	mux := http.NewServeMux()
	tmpl := template.Must(template.ParseFiles("templates/index.html"))

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("GET /api/blocks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blocks.Catalog)
	})

	mux.HandleFunc("GET /api/blocks/html", func(w http.ResponseWriter, r *http.Request) {
		for _, b := range blocks.Catalog {
			fmt.Fprintf(w, `<div draggable="true" data-kind="%s" data-name="%s" class="flex items-center gap-2 px-3 py-2 bg-gray-800 rounded text-sm cursor-grab hover:bg-gray-700 transition-colors select-none">
				<img src="/static/icons/%s.svg" class="w-4 h-4 invert opacity-70" alt="" draggable="false">
				<span>%s</span>
			</div>`, b.Kind, b.Name, b.Kind, b.Name)
		}
	})

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
