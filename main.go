package main

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/davecgh/go-spew/spew"
)

var (
	spentSorting  time.Duration
	spentContains time.Duration
)

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

	neighbors := g.knnSearch(vertexToInsert, 5, k)
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
	// a := &vertex{object: "bag"}
	// tree := &binarySearchTree{}
	// tree.insert(&vertex{object: "foo"}, 0.1)
	// tree.insert(&vertex{object: "bar"}, 0.8)
	// tree.insert(&vertex{object: "baz"}, 0.5)
	// tree.insert(a, 0.6)
	// tree.insert(a, 0.6)
	// tree.insert(&vertex{object: "foz"}, 2.1)
	// tree.insert(&vertex{object: "zof"}, 0.01)

	// tree.printInOrder()
	// fmt.Println(tree.contains(a, 0.6))
	// fmt.Println(tree.contains(&vertex{object: "not contained"}, 0.7))

	// fmt.Println(tree.minimum().data)
	rand.Seed(time.Now().UnixNano())
	vectors := parseVectorsFromFile("./vectors.txt", 1000)
	k := 10

	g := &graph{}

	fmt.Printf("building")
	before := time.Now()
	for i, vector := range vectors {
		g.insert(&vertex{object: vector.object, vector: vector.vector}, k)

		if i%50 == 0 {
			fmt.Printf(".")
		}
	}
	fmt.Printf("\n")

	fmt.Printf("\nsorting: %s\ncontains: %s\ntotal: %s\n\n", spentSorting, spentContains, time.Since(before))
	spentSorting, spentContains = 0, 0

	before = time.Now()
	res := g.knnSearch(&vertex{vector: car}, 5, 15)
	// entry := g.vertices[rand.Intn(len(g.vertices))]
	// res := search(car, entry)
	fmt.Printf("\nsorting: %s\ncontains: %s\ntotal: %s\n\n", spentSorting, spentContains, time.Since(before))
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

// TODO: replace with binary tree for optimal perfomance
func insertOrdered(list []vertexWithDistance, itemToInsert *vertex, query []float32) []vertexWithDistance {
	newItem := vertexWithDistance{
		vertex:   itemToInsert,
		distance: cosineDist(itemToInsert.vector, query),
	}

	for _, current := range list {
		if current.vertex == itemToInsert {
			// is already in the list, nothing to do
			return list
		}
	}

	newList := append(list, newItem)
	before := time.Now()
	sort.Slice(newList, func(a, b int) bool {
		return newList[a].distance < newList[b].distance
	})
	spentSorting += time.Since(before)
	return newList
}

func insertMultipleOrdered(list []vertexWithDistance, itemsToInsert []vertexWithDistance, query []float32) []vertexWithDistance {
	var toInsert []vertexWithDistance
	for _, item := range itemsToInsert {
		if contained(list, item.vertex) {
			continue
		}

		toInsert = append(toInsert, item)
	}

	newList := append(list, toInsert...)
	before := time.Now()
	sort.Slice(newList, func(a, b int) bool {
		return newList[a].distance < newList[b].distance
	})
	spentSorting += time.Since(before)

	return newList
}

func remove(list []vertexWithDistance, itemToRemove *vertex) []vertexWithDistance {
	posToDelete := 0
	for i, current := range list {
		if current.vertex == itemToRemove {
			posToDelete = i
			break
		}
	}

	copy(list[posToDelete:], list[posToDelete+1:])
	// list[len(list)-1] = nil // to avoid mem leak
	list = list[:len(list)-1]

	return list
}

func contained(list []vertexWithDistance, item *vertex) bool {
	before := time.Now()
	defer func() {
		spentContains += time.Since(before)
	}()

	for _, curr := range list {
		if curr.vertex == item {
			return true
		}
	}

	return false
}

type vertexWithDistance struct {
	vertex   *vertex
	distance float32
}

func (g *graph) knnSearch(queryObj *vertex, maximumSearches int, k int) []vertexWithDistance {
	var (
		// TODO: These shouldn't be simply slices, but binary search trees for more
		// efficient adding
		tempRes    []vertexWithDistance
		candidates []vertexWithDistance
		visitedSet []vertexWithDistance
		result     []vertexWithDistance
	)

	for i := 0; i < maximumSearches; i++ {
		entry := g.vertices[rand.Intn(len(g.vertices))]
		candidates = insertOrdered(candidates, entry, queryObj.vector)

		for {
			if len(candidates) == 0 {
				break
			}
			candidate := candidates[0] // there is always at least one and it is ordered by distance, so this is the closest
			candidates = remove(candidates, candidate.vertex)

			if len(result) >= k &&
				candidate.distance > result[k-1].distance {
				break
			}

			for _, friend := range candidate.vertex.edges {
				if !contained(visitedSet, friend) {
					visitedSet = insertOrdered(visitedSet, friend, queryObj.vector)
					tempRes = insertOrdered(tempRes, friend, queryObj.vector)
					candidates = insertOrdered(candidates, friend, queryObj.vector)
				}

			}
		} // end for

		result = insertMultipleOrdered(result, tempRes, queryObj.vector)
	}

	return result[:k]
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
	sim, err := cosineSim(a, b)
	if err != nil {
		panic(err)
	}

	return 1 - sim
}
