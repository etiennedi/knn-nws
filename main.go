package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/boltdb/bolt"
)

type vertex struct {
	object         string
	internalvector []float32
	edges          []*vertex // used by nsw
	edgeLinks      map[int64]struct{}
	index          int64
	sync.RWMutex
}

func (v *vertex) vector() []float32 {
	v.RLock()
	index := v.index
	v.RUnlock()
	vec, err := readVectorFromBolt(index)
	if err != nil {
		panic(err)
	}

	return vec
}

func (v *vertex) String() string {
	v.RLock()
	defer v.RUnlock()
	return v.object
}

var k = 36

type job struct {
	index  int64
	object string
}

func nswWorker(graph *nsw, workerid int, jobs chan job) {
	for job := range jobs {
		graph.insert(&vertex{object: job.object, index: job.index}, k)
	}
}

func hnswWorker(graph *hnsw, workerid int, jobs chan job) {
	for job := range jobs {
		graph.insert(&hnswVertex{id: int(job.index)})
	}
}

var db *bolt.DB

func initBolt() {
	boltdb, err := bolt.Open("./data/bolt.db", 0600, nil)
	if err != nil {
		log.Fatal(err)
	}
	db = boltdb

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte("Vectors"))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

var flagBenchmarkElastic bool
var m *monitoring

func parseFlags() {
	flags := os.Args[1:]
	for _, flag := range flags {
		if flag == "benchmark-elastic" {
			fmt.Println("benchmarking against elasticsearch fast-vector score plugin")
			flagBenchmarkElastic = true
		}
	}
}

func main() {
	startup := time.Now()
	m = newMonitoring()
	initBolt()
	defer db.Close()

	var g = &hnsw{}
	var wordToIndex map[string]int

	if fileExists("./data/hnsw.index") {
		// read hnsw index
		f, err := os.Open("./data/hnsw.index")
		if err != nil {
			log.Fatal(err.Error())
		}

		bytes, err := ioutil.ReadAll(f)
		if err != nil {
			log.Fatal(err.Error())
		}

		err = UnmarshalGzip(bytes, g)
		if err != nil {
			log.Fatal(err.Error())
		}

		g.vectorForID = func(i int) []float32 {
			vec, err := readVectorFromBolt(int64(i))
			if err != nil {
				log.Fatalf(err.Error())
			}
			return vec
		}

		f.Close()

		// read wordToIndex
		f, err = os.Open("./data/object_to_index.json")
		if err != nil {
			log.Fatal(err.Error())
		}

		bytes, err = ioutil.ReadAll(f)
		if err != nil {
			log.Fatal(err.Error())
		}

		err = json.Unmarshal(bytes, &wordToIndex)
		if err != nil {
			log.Fatal(err.Error())
		}

	} else {
		// build new
		g, wordToIndex = buildNewIndex()

	}

	getIndex := func(name string) int64 {
		return int64(wordToIndex[name])
	}

	getData := func(index int64) string {
		// TODO: this can obviously be improved

		for key, value := range wordToIndex {
			if value == int(index) {
				return key
			}
		}

		return ""
	}

	handler := newHandlers(g, getIndex, getData)
	http.Handle("/objects", http.HandlerFunc(handler.getObjects))
	fmt.Printf("Startup took %s, Listening on :8080\n", time.Since(startup))

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}

}

func buildNewIndex() (*hnsw, map[string]int) {
	m.reset()

	limit := 1000

	parseFlags()

	rand.Seed(time.Now().UnixNano())
	if flagBenchmarkElastic {
		err := setMappings()
		if err != nil {
			log.Fatal(err)
		}
	}

	insertFn := func(i int, word string, vector []float32) {
		err := storeToBolt(int64(i), vector)
		if err != nil {
			log.Printf("bolt error: %v\n", err)
		}

		if flagBenchmarkElastic {
			err := storeToES(i, word, vector)
			if err != nil {
				fmt.Printf("es error: %s\n", err)
			}
		}

		if i%100 == 0 {
			fmt.Printf(".")
		}

	}
	wordToIndex := parseVectorsFromFile("./vectors-shuf.txt", limit, insertFn)

	// g := &nsw{}
	g := newHnsw(30, 60, func(i int) []float32 {
		vec, err := readVectorFromBolt(int64(i))
		if err != nil {
			log.Fatalf(err.Error())
		}
		return vec
	})

	m.writeTimes(os.Stdout)
	m.reset()

	fmt.Printf("building index")
	jobs := make(chan job)
	numWorkers := runtime.GOMAXPROCS(0)
	for i := 0; i < numWorkers; i++ {
		fmt.Printf("starting worker %d\n", i)
		// go nswWorker(g, i, jobs)
		go hnswWorker(g, i, jobs)
	}

	start := time.Now()
	indexFn := func(i int, word string, vector []float32) {
		jobs <- job{object: word, index: int64(i)}

		if i%100 == 0 {
			// technically we're measuring the time between jobs we start, not jobs
			// we complete. However, since we only start a new job once an old one
			// has completed, this should be about the same after the first few jobs
			fmt.Printf("last 100 took %s\n", time.Since(start))
			start = time.Now()
		}

	}
	parseVectorsFromFile("./vectors.txt", limit, indexFn)

	// let remaining workers finish and everything calm down

	time.Sleep(3 * time.Second)

	bytes, err := g.MarshalGzip()
	if err != nil {
		log.Printf(err.Error())
	}

	f, err := os.Create("./data/hnsw.index")
	if err != nil {
		log.Printf(err.Error())
	}

	_, err = f.Write(bytes)
	if err != nil {
		log.Printf(err.Error())
	}

	f.Close()

	bytes, err = json.Marshal(wordToIndex)
	if err != nil {
		log.Printf(err.Error())
	}

	f, err = os.Create("./data/object_to_index.json")
	if err != nil {
		log.Printf(err.Error())
	}

	_, err = f.Write(bytes)
	if err != nil {
		log.Printf(err.Error())
	}

	f.Close()

	m.writeTimes(os.Stdout)
	return g, wordToIndex
}

type vertexWithDistance struct {
	vertex   *vertex
	distance float32
}

func cosineSim(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors have different dimensions")
	}

	var (
		sumProduct float64
		sumASquare float64
		sumBSquare float64
	)

	for i := range a {
		sumProduct += float64(a[i] * b[i])
		sumASquare += float64(a[i] * a[i])
		sumBSquare += float64(b[i] * b[i])
	}

	return float32(sumProduct / (math.Sqrt(sumASquare) * math.Sqrt(sumBSquare))), nil
}

func cosineDist(a, b []float32) float32 {
	before := time.Now()
	defer m.addDistancing(before)
	sim, err := cosineSim(a, b)
	if err != nil {
		panic(err)
	}

	return 1 - sim
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
