package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
)

type handlers struct {
	primary   *hnsw
	secondary *hnsw
	getIndex  getIndexFn
	getData   getDataFn
}
type getIndexFn func(name string) int64
type getDataFn func(int64) string

func newHandlers(primary *hnsw, secondary *hnsw, getIndex getIndexFn, getData getDataFn) *handlers {
	return &handlers{primary: primary, secondary: secondary, getIndex: getIndex, getData: getData}
}

func (h *handlers) getObjects(w http.ResponseWriter, r *http.Request) {
	qv := r.URL.Query()
	name := qv.Get("name")
	sizeStr := qv.Get("size")
	_, secondary := qv["secondary"]
	var size int
	if sizeStr == "" {
		size = 15
	} else {
		size, _ = strconv.Atoi(sizeStr)
	}

	indexPos := h.getIndex(name)
	before := time.Now()
	// filter := qv.Get("filter") != ""
	benchmark := qv.Get("benchmark") != ""
	if benchmark {
		h.benchmark(w, r, indexPos, size)
		return
	}

	var g *hnsw
	if secondary {
		fmt.Println("serving from secondary index")
		g = h.secondary
	} else {
		fmt.Println("serving from primary index")
		g = h.primary
	}

	res := g.knnSearch(int(indexPos), size, 100)
	took := time.Since(before)

	results := make([]result, len(res))
	for i, elem := range res {
		object := h.getData(int64(elem))
		results[i] = result{
			Object: object,
			// Distance: elem.distance,
		}
	}

	list := resultsList{
		Results: results,
		Took:    fmt.Sprintf("%s", took),
	}

	json.NewEncoder(w).Encode(list)
}

func (h *handlers) benchmark(w http.ResponseWriter, r *http.Request, indexPos int64, size int) {
	vector, err := readVectorFromBolt(indexPos)
	if err != nil {
		panic(err)
	}

	vectorBytes, _ := json.Marshal(vector)
	body := []byte(fmt.Sprintf(`
{
  "query": {
    "function_score": {
		  "query": {
				"match_all": {}
			},
      "boost_mode": "replace",
      "script_score": {
        "script": {
        "source": "binary_vector_score",
          "lang": "knn",
          "params": {
            "cosine": true,
            "field": "embedding_vector",
            "vector": %s
          }
        }
      }
    }
  },
  "size": %d
}

	`, string(vectorBytes), size))
	fmt.Print(string(body))

	br := bytes.NewBuffer(body)
	res, err := http.Post(fmt.Sprintf("http://localhost:9201/%s/_search", esIndexName), "application/json", br)
	if err != nil {
		w.WriteHeader(500)
		spew.Dump(res.Body)
		return
	}

	resbody, _ := ioutil.ReadAll(res.Body)
	defer res.Body.Close()

	w.Write(resbody)
}

type resultsList struct {
	Took    string   `json:"took"`
	Results []result `json:"results"`
}

type result struct {
	Object   interface{}
	Distance float32
}
