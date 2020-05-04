package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func parseVectorsFromFile(fileName string, limit int, doFn func(i int, word string, vec []float32)) map[string]int {
	fmt.Println("iterating over input files")
	resetTimes()
	file, err := os.Open(fileName)
	defer file.Close()
	if err != nil {
		panic(err)
	}

	out := make(map[string]int)
	scanner := bufio.NewScanner(file)
	i := 0

	for scanner.Scan() {
		if i >= limit {
			break
		}

		row := scanner.Text()
		word, vector := parseVectorRow(row)
		out[word] = i

		doFn(i, word, vector)

		i++
	}
	printTimes()

	return out
}

func parseVectorRow(row string) (string, []float32) {
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

	return word, vector
}
