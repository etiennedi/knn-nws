package main

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Layer refers to the actual slice of graph
// Level is the int index for a Layer, e.g. the topmost layer has level 7

type hnsw struct {
	sync.RWMutex

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

	commitLog *hnswCommitLogger

	// for distributed spike, can be used to call a insertExternal on a different graph
	insertHook func(node, targetLevel int, neighborsAtLevel map[int][]uint32)

	id string
}

// func (h *hnsw) topLevel() int {
// 	return len(h.layers) - 1
// }

type hnswLayer struct{}

func newHnsw(id string, maximumConnections int, efConstruction int, vectorForID func(id int) []float32) *hnsw {
	return &hnsw{
		maximumConnections:          maximumConnections,
		maximumConnectionsLayerZero: 2 * maximumConnections,                    // inspired by original paper and other implementations
		levelNormalizer:             1 / math.Log(float64(maximumConnections)), // inspired by c++ implementation
		efConstruction:              efConstruction,
		nodes:                       make([]*hnswVertex, 0, 100000), // TODO: grow variably rather than fixed length
		vectorForID:                 vectorForID,
		commitLog:                   newHnswCommitLogger(),
		id:                          id,
	}

}

func (h *hnsw) insertFromExternal(nodeId, targetLevel int, neighborsAtLevel map[int][]uint32) {
	defer m.addBuildingReplication(time.Now())

	var node *hnswVertex
	h.RLock()
	total := len(h.nodes)
	if total > nodeId {
		node = h.nodes[nodeId] // it could be that we implicitly added this node already because it was referenced
	}
	h.RUnlock()

	if node == nil {
		node = &hnswVertex{
			id:          nodeId,
			connections: make(map[int][]uint32),
			level:       targetLevel,
		}
	} else {
		node.level = targetLevel
	}

	if total == 0 {
		h.Lock()
		h.commitLog.SetEntryPointWithMaxLayer(node.id, 0)
		h.entryPointID = node.id
		h.currentMaximumLayer = 0
		node.connections = map[int][]uint32{}
		node.level = 0
		h.nodes = make([]*hnswVertex, 100000)
		h.commitLog.AddNode(node)
		h.nodes[node.id] = node
		h.Unlock()
		return
	}

	currentMaximumLayer := h.currentMaximumLayer
	h.Lock()
	h.nodes[nodeId] = node
	h.commitLog.AddNode(node)
	h.Unlock()

	for level := min(targetLevel, currentMaximumLayer); level >= 0; level-- {
		neighbors := neighborsAtLevel[level]

		for _, neighborID := range neighbors {
			h.RLock()
			neighbor := h.nodes[neighborID]
			if neighbor == nil {
				// due to everything being parallel it could be that the linked neighbor
				// doesn't exist yet
				h.nodes[neighborID] = &hnswVertex{
					id:          int(neighborID),
					connections: make(map[int][]uint32),
				}
				neighbor = h.nodes[neighborID]
			}
			h.RUnlock()

			neighbor.linkAtLevel(level, uint32(nodeId), h.commitLog)
			node.linkAtLevel(level, uint32(neighbor.id), h.commitLog)

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
			h.commitLog.ReplaceLinksAtLevel(neighbor.id, level, updatedConnections)
			neighbor.connections[level] = updatedConnections
			neighbor.Unlock()
		}
	}

	if targetLevel > h.currentMaximumLayer {
		h.Lock()
		h.commitLog.SetEntryPointWithMaxLayer(nodeId, targetLevel)
		h.entryPointID = nodeId
		h.currentMaximumLayer = targetLevel
		h.Unlock()
	}

}

func (h *hnsw) insert(node *hnswVertex) {
	before := time.Now()
	h.RLock()
	m.addBuildingReadLockingBeginning(before)
	total := len(h.nodes)
	h.RUnlock()

	if total == 0 {
		h.Lock()
		h.commitLog.SetEntryPointWithMaxLayer(node.id, 0)
		h.entryPointID = node.id
		h.currentMaximumLayer = 0
		node.connections = map[int][]uint32{}
		node.level = 0
		h.nodes = make([]*hnswVertex, 100000)
		h.commitLog.AddNode(node)
		h.nodes[node.id] = node
		h.Unlock()

		go h.insertHook(node.id, 0, node.connections)
		return
	}
	// initially use the "global" entrypoint which is guaranteed to be on the
	// currently highest layer
	entryPointID := h.entryPointID

	// initially use the level of the entrypoint which is the highest level of
	// the h-graph in the first iteration
	currentMaximumLayer := h.currentMaximumLayer

	targetLevel := int(math.Floor(-math.Log(rand.Float64()*h.levelNormalizer))) - 1

	before = time.Now()
	node.Lock()
	m.addBuildingItemLocking(before)
	node.level = targetLevel
	node.connections = map[int][]uint32{}
	node.Unlock()

	before = time.Now()
	h.Lock()
	m.addBuildingLocking(before)
	nodeId := node.id
	h.nodes[nodeId] = node
	h.commitLog.AddNode(node)
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

	neighborsAtLevel := make(map[int][]uint32) // for distributed spike

	for level := min(targetLevel, currentMaximumLayer); level >= 0; level-- {
		results = h.searchLayer(node, *results, h.efConstruction, level)

		// TODO: support both neighbor selection algos
		neighbors := h.selectNeighborsSimple(nodeId, *results, h.maximumConnections)

		// for distributed spike
		neighborsAtLevel[level] = neighbors

		for _, neighborID := range neighbors {
			before := time.Now()
			h.RLock()
			m.addBuildingReadLocking(before)
			neighbor := h.nodes[neighborID]
			h.RUnlock()

			neighbor.linkAtLevel(level, uint32(nodeId), h.commitLog)
			node.linkAtLevel(level, uint32(neighbor.id), h.commitLog)

			before = time.Now()
			neighbor.RLock()
			m.addBuildingItemLocking(before)
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

			before = time.Now()
			neighbor.Lock()
			m.addBuildingItemLocking(before)
			h.commitLog.ReplaceLinksAtLevel(neighbor.id, level, updatedConnections)
			neighbor.connections[level] = updatedConnections
			neighbor.Unlock()
		}
	}

	go h.insertHook(nodeId, targetLevel, neighborsAtLevel)

	if targetLevel > h.currentMaximumLayer {
		before = time.Now()
		h.Lock()
		m.addBuildingLocking(before)
		h.commitLog.SetEntryPointWithMaxLayer(nodeId, targetLevel)
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

		before := time.Now()
		h.RLock()
		m.addBuildingReadLocking(before)
		candidateNode := h.nodes[candidate.index]
		h.RUnlock()

		before = time.Now()
		candidateNode.RLock()
		m.addBuildingItemLocking(before)
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

func (v *hnswVertex) linkAtLevel(level int, target uint32, cl *hnswCommitLogger) {
	v.Lock()
	cl.AddLinkAtLevel(v.id, level, target)
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

func (h *hnsw) Stats() {
	fmt.Printf("levels: %d\n", h.currentMaximumLayer)

	perLevelCount := map[int]uint{}

	for _, node := range h.nodes {
		if node == nil {
			continue
		}
		l := node.level
		if l == 0 && len(node.connections) == 0 {
			// filter out allocated space without nodes
			continue
		}
		c, ok := perLevelCount[l]
		if !ok {
			perLevelCount[l] = 0
		}

		perLevelCount[l] = c + 1
	}

	for level, count := range perLevelCount {
		fmt.Printf("unique count on level %d: %d\n", level, count)
	}

}
