package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/prashanth/archimedes/internal/blocks"
	_ "github.com/prashanth/archimedes/internal/blocks/cache"
	_ "github.com/prashanth/archimedes/internal/blocks/datastore"
	_ "github.com/prashanth/archimedes/internal/blocks/queue"
	_ "github.com/prashanth/archimedes/internal/blocks/search"
	"github.com/prashanth/archimedes/internal/engine"
)

func main() {
	mux := http.NewServeMux()
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	sim := engine.NewSim()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, nil)
	})

	mux.HandleFunc("GET /api/blocks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		type entry struct {
			Kind string `json:"kind"`
			Name string `json:"name"`
		}
		out := make([]entry, len(blocks.Types))
		for i, b := range blocks.Types {
			out[i] = entry{b.Kind(), b.Name()}
		}
		json.NewEncoder(w).Encode(out)
	})

	mux.HandleFunc("GET /api/blocks/html", func(w http.ResponseWriter, r *http.Request) {
		for _, b := range blocks.Types {
			fmt.Fprintf(w, `<div draggable="true" data-kind="%s" data-name="%s" class="flex items-center gap-2 px-3 py-2 bg-gray-800 rounded text-sm cursor-grab hover:bg-gray-700 transition-colors select-none">
				<img src="/static/icons/%s.svg" class="w-4 h-4 invert opacity-70" alt="" draggable="false">
				<span>%s</span>
			</div>`, b.Kind(), b.Name(), b.Kind(), b.Name())
		}
	})

	mux.HandleFunc("POST /api/topology", func(w http.ResponseWriter, r *http.Request) {
		var topo engine.Topology
		if err := json.NewDecoder(r.Body).Decode(&topo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		g, err := engine.BuildGraph(topo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		results, err := engine.Simulate(g, topo.RPS, topo.ReadRatio)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"blocks": results})
	})

	mux.HandleFunc("POST /api/play", func(w http.ResponseWriter, r *http.Request) {
		var topo engine.Topology
		if err := json.NewDecoder(r.Body).Decode(&topo); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := sim.Play(topo); err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/pause", func(w http.ResponseWriter, r *http.Request) {
		sim.Pause()
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("POST /api/rps", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			RPS       float64 `json:"rps"`
			ReadRatio float64 `json:"read_ratio"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sim.UpdateRPS(body.RPS, body.ReadRatio)
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := sim.Subscribe()
		defer sim.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case tr := <-ch:
				data, _ := json.Marshal(tr)
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
