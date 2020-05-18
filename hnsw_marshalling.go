package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

func (h *hnsw) MarshalGzip() ([]byte, error) {
	ec := &errorCompounder{}
	b := bytes.NewBuffer(nil)

	ec.add(h.writeAsInt64(b, h.maximumConnections))
	ec.add(h.writeAsInt64(b, h.maximumConnectionsLayerZero))
	ec.add(h.writeAsInt64(b, h.currentMaximumLayer))
	ec.add(h.writeAsInt64(b, h.entryPointID))
	ec.add(h.writeAsInt64(b, h.efConstruction))
	ec.add(h.writeFloat64(b, h.levelNormalizer))

	ec.add(h.writeAsInt64(b, len(h.nodes)))
	for _, node := range h.nodes {
		if node == nil {
			// in case we grew further than what we actually need
			continue
		}
		ec.add(h.writeAsInt64(b, node.id))
		ec.add(h.writeAsInt64(b, node.level))
		connectionLevels := len(node.connections)
		ec.add(h.writeAsInt64(b, connectionLevels))
		for level, conns := range node.connections {
			ec.add(h.writeAsInt64(b, level))
			connectionsLength := len(conns)
			ec.add(h.writeAsInt64(b, connectionsLength))
			ec.add(h.writeUint32Slice(b, conns))
		}
	}

	if len(ec.errors) != 0 {
		return nil, fmt.Errorf("%v", ec.errors)
	}

	return b.Bytes(), nil
}

func (h *hnsw) writeAsInt64(w io.Writer, in int) error {
	typed := int64(in)
	err := binary.Write(w, binary.LittleEndian, &typed)
	if err != nil {
		return fmt.Errorf("writing int64: %v", err)
	}

	return nil
}

func (h *hnsw) writeFloat64(w io.Writer, in float64) error {
	err := binary.Write(w, binary.LittleEndian, &in)
	if err != nil {
		return fmt.Errorf("writing float64: %v", err)
	}

	return nil
}

func (h *hnsw) writeUint32Slice(w io.Writer, in []uint32) error {
	err := binary.Write(w, binary.LittleEndian, &in)
	if err != nil {
		return fmt.Errorf("writing []uint32: %v", err)
	}

	return nil
}

func UnmarshalGzip(in []byte, g *hnsw) error {
	ec := &errorCompounder{}
	b := bytes.NewBuffer(in)
	var err error

	g.maximumConnections, err = g.readFromInt64(b)
	ec.add(err)
	g.maximumConnectionsLayerZero, err = g.readFromInt64(b)
	ec.add(err)
	g.currentMaximumLayer, err = g.readFromInt64(b)
	ec.add(err)
	g.entryPointID, err = g.readFromInt64(b)
	ec.add(err)
	g.efConstruction, err = g.readFromInt64(b)
	ec.add(err)
	g.levelNormalizer, err = g.readFloat64(b)
	ec.add(err)

	lenNodes, err := g.readFromInt64(b)
	ec.add(err)

	g.nodes = make([]*hnswVertex, lenNodes)
	for i := range g.nodes {
		node := hnswVertex{}
		node.id, err = g.readFromInt64(b)
		ec.add(err)

		node.level, err = g.readFromInt64(b)
		ec.add(err)

		levelsLength, err := g.readFromInt64(b)
		ec.add(err)

		node.connections = map[int][]uint32{}
		for i := levelsLength; i > 0; i-- {
			level, err := g.readFromInt64(b)
			ec.add(err)

			connectionsLength, err := g.readFromInt64(b)
			ec.add(err)

			connections, err := g.readUint32Slice(b, connectionsLength)
			ec.add(err)

			node.connections[level] = connections
		}

		g.nodes[i] = &node
	}

	return nil
}

func (h *hnsw) readFromInt64(r io.Reader) (int, error) {
	var value int64
	err := binary.Read(r, binary.LittleEndian, &value)
	if err != nil {
		return 0, fmt.Errorf("reading int64: %v", err)
	}

	return int(value), nil
}

func (h *hnsw) readFloat64(r io.Reader) (float64, error) {
	var value float64
	err := binary.Read(r, binary.LittleEndian, &value)
	if err != nil {
		return 0, fmt.Errorf("reading float64: %v", err)
	}

	return value, nil
}

func (h *hnsw) readUint32Slice(r io.Reader, length int) ([]uint32, error) {
	value := make([]uint32, length)
	err := binary.Read(r, binary.LittleEndian, &value)
	if err != nil {
		return nil, fmt.Errorf("reading []uint32: %v", err)
	}

	return value, nil
}

type errorCompounder struct {
	errors []error
}

func (e *errorCompounder) add(err error) {
	if err != nil {
		e.errors = append(e.errors, err)
	}
}
