package main

import (
	"math"
	"math/rand"
	"sync"
)

// Layer refers to the actual slice of graph
// Level is the int index for a Layer, e.g. the topmost layer has level 7

type hnsw struct {
	sync.RWMutex
	// layers []hnswLayer
	vertices []*hnswVertex

	// Each node should not have more edges than this number
	maximumConnections int

	// Nodes in the lowest level have a separate (usually higher) max connection
	// limit
	maximumConnectionsLayerZero int

	// the current maximum can be smaller than the configured maximum because of
	// the exponentially decaying layer function. The initial entry is started at
	// layer 0, but this has the chance to grow with every subsequent entry
	currentMaximumLayer int

	// this is a point on the highest level, if we insert a new point with a
	// higher level it will become the new entry point. Note tat the level of
	// this point is always currentMaximumLayer
	entryPointID int

	// ef parameter used in construction phases, should be higher than ef during querying
	efConstruction int

	levelNormalizer float64

	nodes []*hnswVertex

	vectorForID func(id int) []float32
}

// func (h *hnsw) topLevel() int {
// 	return len(h.layers) - 1
// }

type hnswLayer struct{}

func newHnsw(maximumConnections int, efConstruction int, vectorForID func(id int) []float32) *hnsw {
	return &hnsw{
		maximumConnections:          maximumConnections,
		maximumConnectionsLayerZero: 2 * maximumConnections,                    // inspired by original paper and other implementations
		levelNormalizer:             1 / math.Log(float64(maximumConnections)), // inspired by c++ implementation
		efConstruction:              efConstruction,
		nodes:                       make([]*hnswVertex, 0, 10000), // TODO: grow variably rather than fixed length
		vectorForID:                 vectorForID,
	}

}

func (h *hnsw) insert(node *hnswVertex) {

	h.RLock()
	total := len(h.nodes)
	h.RUnlock()

	if total == 0 {
		h.Lock()
		h.entryPointID = node.id
		node.connections = map[int][]uint32{}
		node.level = 0
		h.nodes = make([]*hnswVertex, 10000)
		h.nodes[node.id] = node
		h.currentMaximumLayer = 0
		h.Unlock()
		return
	}
	// initially use the "global" entrypoint which is guaranteed to be on the
	// currently highest layer
	entryPointID := h.entryPointID

	// initially use the level of the entrypoint which is the highest level of
	// the h-graph in the first iteration
	currentMaximumLayer := h.currentMaximumLayer

	targetLevel := int(math.Floor(-math.Log(rand.Float64() * h.levelNormalizer)))

	node.Lock()
	node.level = targetLevel
	node.connections = map[int][]uint32{}
	node.Unlock()

	h.Lock()
	nodeId := node.id
	h.nodes[nodeId] = node
	h.Unlock()

	// in case the new target is lower than the current max, we need to search
	// each layer for a better candidate and update the candidate
	for level := currentMaximumLayer; level > targetLevel; level-- {
		tmpBST := &binarySearchTreeGeneric{}
		tmpBST.insert(entryPointID, h.distBetweenNodes(nodeId, entryPointID))
		res := h.searchLayer(node, *tmpBST, 1, level)
		entryPointID = res.minimum().index
	}

	var results = &binarySearchTreeGeneric{}
	results.insert(entryPointID, h.distBetweenNodes(nodeId, entryPointID))

	for level := min(targetLevel, currentMaximumLayer); level >= 0; level-- {
		results = h.searchLayer(node, *results, h.efConstruction, level)

		// TODO: support both neighbor selection algos
		neighbors := h.selectNeighborsSimple(nodeId, *results, h.maximumConnections)

		for _, neighborID := range neighbors {
			h.RLock()
			neighbor := h.nodes[neighborID]
			h.RUnlock()

			neighbor.linkAtLevel(level, uint32(nodeId))
			node.linkAtLevel(level, uint32(neighbor.id))

			neighbor.RLock()
			currentConnections := neighbor.connections[level]
			neighbor.RUnlock()

			maximumConnections := h.maximumConnections
			if level == 0 {
				maximumConnections = h.maximumConnectionsLayerZero
			}

			if len(currentConnections) <= maximumConnections {
				// nothing to do, skip
				continue
			}

			// TODO: support both neighbor selection algos
			updatedConnections := h.selectNeighborsSimpleFromId(nodeId, currentConnections, maximumConnections)

			neighbor.Lock()
			neighbor.connections[level] = updatedConnections
			neighbor.Unlock()
		}
	}

	if targetLevel > h.currentMaximumLayer {
		h.Lock()
		h.entryPointID = nodeId
		h.currentMaximumLayer = targetLevel
		h.Unlock()
	}
}

func (h *hnsw) searchLayer(queryNode *hnswVertex, entrypoints binarySearchTreeGeneric, ef int, level int) *binarySearchTreeGeneric {

	// create 3 copies of the entrypoint bst
	visited := map[uint32]struct{}{}
	for _, elem := range entrypoints.flattenInOrder() {
		visited[uint32(elem.index)] = struct{}{}
	}
	candidates := &binarySearchTreeGeneric{}
	results := &binarySearchTreeGeneric{}

	for _, ep := range entrypoints.flattenInOrder() {
		candidates.insert(ep.index, ep.dist)
		results.insert(ep.index, ep.dist)
	}

	for candidates.root != nil { // efficient way to see if the len is > 0
		candidate := candidates.minimum()
		candidates.delete(candidate.index, candidate.dist)
		worstResultDistance := h.distBetweenNodes(results.maximum().index, queryNode.id)

		if h.distBetweenNodes(candidate.index, queryNode.id) > worstResultDistance {
			break
		}

		h.RLock()
		candidateNode := h.nodes[candidate.index]
		h.RUnlock()

		candidateNode.RLock()
		connections := candidateNode.connections[level]
		candidateNode.RUnlock()

		for _, neighborID := range connections {
			if _, ok := visited[neighborID]; ok {
				// skip if we've already visited this neighbor
				continue
			}

			// make sure we never visit this neighbor again
			visited[neighborID] = struct{}{}

			distance := h.distBetweenNodes(int(neighborID), queryNode.id)
			resLenBefore := results.len() // calculating just once saves a bit of time
			if distance < worstResultDistance || resLenBefore < ef {
				results.insert(int(neighborID), distance)
				candidates.insert(int(neighborID), distance)

				if resLenBefore+1 > ef { // +1 because we have added one node size calculating the len
					max := results.maximum()
					results.delete(max.index, max.dist)
				}

			}

		}
	}

	return results
}

func (h *hnsw) selectNeighborsSimple(nodeId int, input binarySearchTreeGeneric, max int) []uint32 {
	flat := input.flattenInOrder()
	size := min(len(flat), max)
	out := make([]uint32, size)
	for i, elem := range flat {
		if i >= size {
			break
		}
		out[i] = uint32(elem.index)
	}

	return out
}

func (h *hnsw) selectNeighborsSimpleFromId(nodeId int, ids []uint32, max int) []uint32 {
	bst := &binarySearchTreeGeneric{}
	for _, id := range ids {
		dist := h.distBetweenNodes(int(id), nodeId)
		bst.insert(int(id), dist)
	}

	return h.selectNeighborsSimple(nodeId, *bst, max)
}

func (v *hnswVertex) linkAtLevel(level int, target uint32) {
	v.Lock()
	v.connections[level] = append(v.connections[level], target)
	v.Unlock()
}

type hnswVertex struct {
	id int
	sync.RWMutex
	level       int
	connections map[int][]uint32 // map[level][]connectedId
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *hnsw) distBetweenNodes(a, b int) float32 {
	return cosineDist(h.vectorForID(a), h.vectorForID(b))
}

func (h *hnsw) knnSearch(queryNodeID int, k int, ef int) []int {
	h.RLock()
	queryNode := h.nodes[queryNodeID]
	h.RUnlock()

	entryPointID := h.entryPointID
	entryPointDistance := h.distBetweenNodes(entryPointID, queryNodeID)

	for level := h.currentMaximumLayer; level >= 1; level-- { // stop at layer 1, not 0!
		eps := &binarySearchTreeGeneric{}
		eps.insert(entryPointID, entryPointDistance)
		res := h.searchLayer(queryNode, *eps, 1, level)
		best := res.minimum()
		entryPointID = best.index
		entryPointDistance = best.dist
	}

	eps := &binarySearchTreeGeneric{}
	eps.insert(entryPointID, entryPointDistance)
	res := h.searchLayer(queryNode, *eps, ef, 0)

	flat := res.flattenInOrder()
	size := min(len(flat), k)
	out := make([]int, size)
	for i, elem := range flat {
		if i >= size {
			break
		}
		out[i] = elem.index
	}

	return out
}
