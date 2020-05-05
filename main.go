package main

import (
	"fmt"
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

var (
	startTime        time.Time
	spentInserting   time.Duration
	spentContains    time.Duration
	spentFlattening  time.Duration
	spentDeleting    time.Duration
	spentDistancing  time.Duration
	spentReadingDisk time.Duration
)

func resetTimes() {
	spentInserting = 0
	spentContains = 0
	spentFlattening = 0
	spentDeleting = 0
	spentDistancing = 0
	spentReadingDisk = 0
	startTime = time.Now()
}

func printTimes() {
	fmt.Printf(`
inserting: %s
contains: %s
flattening: %s
deleting: %s
distancing: %s
reading disk: %s
total: %s
`, spentInserting, spentContains, spentFlattening, spentDeleting,
		spentDistancing, spentReadingDisk, time.Since(startTime))
}

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

func worker(graph *nsw, id int, jobs chan job) {
	for job := range jobs {
		graph.insert(&vertex{object: job.object, index: job.index}, k)
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
	limit := 200

	parseFlags()
	initBolt()
	defer db.Close()

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

	g := &nsw{}

	fmt.Printf("building index")
	jobs := make(chan job)
	numWorkers := runtime.GOMAXPROCS(0)
	for i := 0; i < numWorkers; i++ {
		fmt.Printf("starting worker %d\n", i)
		go worker(g, i, jobs)
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
	parseVectorsFromFile("./vectors-shuf.txt", limit, indexFn)

	// let remaining workers finish and everything calm down

	resetTimes()

	getIndex := func(name string) int64 {
		return int64(wordToIndex[name])
	}

	handler := newHandlers(g, getIndex)
	http.Handle("/objects", http.HandlerFunc(handler.getObjects))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}

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
	sim, err := cosineSim(a, b)
	if err != nil {
		panic(err)
	}

	spentDistancing += time.Since(before)
	// fmt.Printf("dist %f\n", sim)
	return 1 - sim
}
