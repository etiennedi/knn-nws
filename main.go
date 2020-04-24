package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/davecgh/go-spew/spew"
)

var (
	startTime       time.Time
	spentInserting  time.Duration
	spentContains   time.Duration
	spentFlattening time.Duration
	spentDeleting   time.Duration
	spentDistancing time.Duration
)

func resetTimes() {
	spentInserting = 0
	spentContains = 0
	spentFlattening = 0
	spentDeleting = 0
	spentDistancing = 0
	startTime = time.Now()
}

func printTimes() {
	fmt.Printf(`
inserting: %s
contains: %s
flattening: %s
deleting: %s
distancing: %s
total: %s
`, spentInserting, spentContains, spentFlattening, spentDeleting, spentDistancing, time.Since(startTime))
}

type vertex struct {
	object string
	vector []float32
	edges  []*vertex
}

func (v vertex) String() string {
	return v.object
}

type graph struct {
	vertices []*vertex
}

func (g *graph) insert(vertexToInsert *vertex, k int) {
	if len(g.vertices) == 0 {
		// nothing to connect, simply insert
		g.vertices = []*vertex{vertexToInsert}
		return
	}

	if len(g.vertices) < k {
		// insert and connect to everything
		for i, elem := range g.vertices {
			elem.edges = append(elem.edges, vertexToInsert)
			g.vertices[i] = elem
			vertexToInsert.edges = append(vertexToInsert.edges, g.vertices[i])
		}

		g.vertices = append(g.vertices, vertexToInsert)
		return
	}

	neighbors := g.knnSearch(vertexToInsert, 1, k)
	for _, neighbor := range neighbors {
		neighbor.vertex.edges = append(neighbor.vertex.edges, vertexToInsert)
		vertexToInsert.edges = append(vertexToInsert.edges, neighbor.vertex)
	}
	g.vertices = append(g.vertices, vertexToInsert)
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

func main() {
	rand.Seed(time.Now().UnixNano())
	vectors := parseVectorsFromFile("./vectors.txt", 10000)
	k := 50

	g := &graph{}

	resetTimes()
	fmt.Printf("building")
	start := time.Now()
	for i, vector := range vectors {
		g.insert(&vertex{object: vector.object, vector: vector.vector}, k)

		if i%50 == 0 {
			fmt.Printf("last 50 took %s\n", time.Since(start))
			start = time.Now()
		}
	}
	fmt.Printf("\n")

	printTimes()

	resetTimes()
	res := g.knnSearch(&vertex{vector: car}, 1, 15)
	// entry := g.vertices[rand.Intn(len(g.vertices))]
	// res := search(car, entry)
	printTimes()
	spew.Dump(res)

}

func search(query []float32, entryPoint *vertex) *vertex {
	var current *vertex
	var next *vertex
	var minDist float32

	current = entryPoint
	minDist = cosineDist(current.vector, query)

	for _, friend := range current.edges {
		friendDist := cosineDist(friend.vector, query)
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

func (g *graph) knnSearch(queryObj *vertex, maximumSearches int, k int) []vertexWithDistance {
	var (
		tempRes    = &binarySearchTree{}
		candidates = &binarySearchTree{}
		visitedSet = &binarySearchTree{}
		result     = &binarySearchTree{}
	)

	for i := 0; i < maximumSearches; i++ {
		entry := g.vertices[rand.Intn(len(g.vertices))]
		candidates.insert(entry, cosineDist(queryObj.vector, entry.vector))

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
				friendDist := cosineDist(friend.vector, queryObj.vector)
				if !visitedSet.contains(friend, friendDist) {
					visitedSet.insert(friend, friendDist)
					tempRes.insert(friend, friendDist)
					candidates.insert(friend, friendDist)
				}
			}
		} // end for

		for _, elem := range tempRes.flattenInOrder() {
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
	return 1 - sim
}
