package main

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

func parseVectorsFromFile(fileName string, limit int) []vertex {
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	out := make([]vertex, limit)
	scanner := bufio.NewScanner(file)
	i := 0

	for scanner.Scan() {
		if i >= limit {
			break
		}

		row := scanner.Text()
		out[i] = parseVectorRow(row)
		i++
	}

	return out
}

func parseVectorRow(row string) vertex {
	parts := strings.Split(row, " ")
	word := parts[0]
	vectorDimensions := parts[1:]

	vector := make([]float32, len(vectorDimensions))
	for i, dim := range vectorDimensions {
		parsed, err := strconv.ParseFloat(dim, 32)
		if err != nil {
			panic(err)
		}

		vector[i] = float32(parsed)
	}

	return vertex{object: word, vector: vector}
}
