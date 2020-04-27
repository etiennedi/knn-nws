package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"runtime"
	"sync"
	"time"
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
	edges          []*vertex
	index          int64
	sync.RWMutex
}

func (v *vertex) vector() []float32 {
	v.RLock()
	index := v.index
	v.RUnlock()
	vec, err := readVectorFromFile(index)
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

type graph struct {
	sync.RWMutex
	vertices []*vertex
}

func (g *graph) insert(vertexToInsert *vertex, k int) {
	g.RLock()
	currentLen := len(g.vertices)
	g.RUnlock()

	if currentLen == 0 {
		// nothing to connect, simply insert
		g.Lock()
		g.vertices = []*vertex{vertexToInsert}
		g.Unlock()
		return
	}

	if currentLen < k {
		// this is only true in the very beginning, we can lock the entire graph
		// without any performance penalty as this case will only run a max of k
		// times
		g.Lock()
		// insert and connect to everything

		for i, elem := range g.vertices {
			elem.edges = append(elem.edges, vertexToInsert)
			g.vertices[i] = elem
			vertexToInsert.edges = append(vertexToInsert.edges, g.vertices[i])
		}

		g.vertices = append(g.vertices, vertexToInsert)
		g.Unlock()
		return
	}

	neighbors := g.knnSearch(vertexToInsert, 1, k, false)
	for _, neighbor := range neighbors {
		neighbor.vertex.Lock()
		neighbor.vertex.edges = append(neighbor.vertex.edges, vertexToInsert)
		neighbor.vertex.Unlock()

		vertexToInsert.edges = append(vertexToInsert.edges, neighbor.vertex)
	}

	g.Lock()
	g.vertices = append(g.vertices, vertexToInsert)
	g.Unlock()
}

func (g *graph) print() {
	for _, vertex := range g.vertices {
		fmt.Printf("%s\n", vertex.object)
		for _, edge := range vertex.edges {
			fmt.Printf("  - %s\n", edge.object)
		}

		fmt.Printf("\n\n")
	}
}

var k = 36

type job struct {
	index  int64
	object string
}

func worker(graph *graph, id int, jobs chan job) {
	for job := range jobs {
		graph.insert(&vertex{object: job.object, index: job.index}, k)
	}
}

func main() {
	rand.Seed(time.Now().UnixNano())
	vectors := parseVectorsFromFile("./vectors.txt", 10000)

	g := &graph{}

	fmt.Printf("building")
	// TODO: don't use actual vertex structure here, it's just a helper and we don't need the lock
	for i, vector := range vectors {
		vectors[i].index = int64(i)
		err := storeToFile(int64(i), vector.internalvector)
		if err != nil {
			log.Fatal(err)
		}
	}

	initMagicMappedFile()

	// start := time.Now()
	// TODO: don't use actual vertex structure here, it's just a helper and we don't need the lock

	jobs := make(chan job)
	numWorkers := runtime.GOMAXPROCS(0)
	for i := 0; i < numWorkers; i++ {
		fmt.Printf("starting worker %d\n", i)
		go worker(g, i, jobs)
	}

	start := time.Now()
	for i, vector := range vectors {
		jobs <- job{object: vector.object, index: vector.index}

		if i%100 == 0 {
			// technically we're measuring the time between jobs we start, not jobs
			// we complete. However, since we only start a new job once an old one
			// has completed, this should be about the same after the first few jobs
			fmt.Printf("last 100 took %s\n", time.Since(start))
			start = time.Now()
		}
	}

	printTimes()

	// let remaining workers finish and everything calm down

	resetTimes()

	getIndex := func(name string) int64 {
		for _, vec := range vectors {
			if vec.object == name {
				return vec.index
			}
		}
		return -1
	}

	handler := newHandlers(g, getIndex)
	http.Handle("/objects", http.HandlerFunc(handler.getObjects))
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}

}

func search(query []float32, entryPoint *vertex) *vertex {
	var current *vertex
	var next *vertex
	var minDist float32
	var vectorCache map[string][]float32

	getVector := func(vertex *vertex) []float32 {
		vec, ok := vectorCache[vertex.object]
		if !ok {
			vec := vertex.vector()
			vectorCache[vertex.object] = vertex.vector()
			return vec
		}

		return vec
	}

	current = entryPoint
	minDist = cosineDist(getVector(current), query)

	for _, friend := range current.edges {
		friendDist := cosineDist(getVector(friend), query)
		if friendDist < minDist {
			minDist = friendDist
			next = friend
		}
	}

	if next == nil {
		return current
	}

	return search(query, next)
}

type vertexWithDistance struct {
	vertex   *vertex
	distance float32
}

func (g *graph) knnSearch(queryObj *vertex, maximumSearches int, k int, filter bool) []vertexWithDistance {
	var (
		tempRes     = &binarySearchTree{}
		candidates  = &binarySearchTree{}
		visitedSet  = &binarySearchTree{}
		result      = &binarySearchTree{}
		vectorCache = map[string][]float32{}
	)
	getVector := func(vertex *vertex) []float32 {
		vec, ok := vectorCache[vertex.object]
		if !ok {
			vec := vertex.vector()
			vectorCache[vertex.object] = vertex.vector()
			return vec
		}

		return vec
	}

	for i := 0; i < maximumSearches; i++ {
		g.RLock()
		entry := g.vertices[rand.Intn(len(g.vertices))]
		g.RUnlock()

		candidates.insert(entry, cosineDist(getVector(queryObj), getVector(entry)))

		hops := 0
		for {
			hops++
			if candidates.root == nil {
				break
			}
			candidate := candidates.minimum()
			candidateData := candidate.data
			candidateDist := candidate.dist
			candidates.delete(candidateData, candidateDist)

			resultSlice := tempRes.flattenInOrder()
			if len(resultSlice) >= k &&
				candidateDist > resultSlice[k-1].dist {
				break
			}

			for _, friend := range candidateData.edges {
				friendDist := cosineDist(getVector(friend), getVector(queryObj))
				if !visitedSet.contains(friend, friendDist) {
					visitedSet.insert(friend, friendDist)
					tempRes.insert(friend, friendDist)
					candidates.insert(friend, friendDist)
				}
			}
		} // end for

		for _, elem := range tempRes.flattenInOrder() {
			// being attempt to filter
			if filter {
				if elem.data.object[0] != "a"[0] {
					continue
				}
			}

			// end filter

			result.insert(elem.data, elem.dist)
		}
	}

	out := make([]vertexWithDistance, k)
	results := result.flattenInOrder()
	for i := range out {
		out[i] = vertexWithDistance{
			vertex:   results[i].data,
			distance: results[i].dist,
		}
	}

	return out
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
