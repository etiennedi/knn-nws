package main

import (
	"fmt"
	"math/rand"
	"sync"
)

type nsw struct {
	sync.RWMutex
	vertices []*vertex
}

func (g *nsw) insert(vertexToInsert *vertex, k int) {
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

func (g *nsw) print() {
	for _, vertex := range g.vertices {
		fmt.Printf("%s\n", vertex.object)
		for _, edge := range vertex.edges {
			fmt.Printf("  - %s\n", edge.object)
		}

		fmt.Printf("\n\n")
	}
}

func (g *nsw) knnSearch(queryObj *vertex, maximumSearches int, k int, filter bool) []vertexWithDistance {
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

func (g *nsw) search(query []float32, entryPoint *vertex) *vertex {
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

	return g.search(query, next)
}
