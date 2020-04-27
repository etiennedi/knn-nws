package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type handlers struct {
	graph    *graph
	getIndex getIndexFn
}
type getIndexFn func(name string) int64

func newHandlers(g *graph, getIndex getIndexFn) *handlers {
	return &handlers{graph: g, getIndex: getIndex}
}

func (h *handlers) getObjects(w http.ResponseWriter, r *http.Request) {
	qv := r.URL.Query()
	name := qv.Get("name")
	indexPos := h.getIndex(name)
	before := time.Now()
	filter := qv.Get("filter") != ""
	res := h.graph.knnSearch(&vertex{index: indexPos}, 1, 10, filter)
	took := time.Since(before)

	results := make([]result, len(res))
	for i, elem := range res {
		results[i] = result{
			Object:   elem.vertex.object,
			Distance: elem.distance,
		}
	}

	list := resultsList{
		Results: results,
		Took:    fmt.Sprintf("%s", took),
	}

	json.NewEncoder(w).Encode(list)
}

type resultsList struct {
	Took    string   `json:"took"`
	Results []result `json:"results"`
}

type result struct {
	Object   interface{}
	Distance float32
}
