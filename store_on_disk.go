package main

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math"
	"os"
)

func storeFile(name string, vector []float32) error {
	f, err := os.Create(fmt.Sprintf("./data/%s", name))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(vectorToBytes(vector))
	if err != nil {
		return err
	}
	return nil
}

func vectorToBytes(in []float32) []byte {

	bytes := make([]byte, len(in)*4)
	i := 0
	for _, elem := range in {
		bits := math.Float32bits(elem)
		binary.LittleEndian.PutUint32(bytes[i:i+4], bits)
		i += 4
	}
	return bytes
}

func vectorFromBytes(in []byte) ([]float32, error) {
	if len(in)%4 != 0 {
		return nil, fmt.Errorf("impossible byte length %d", len(in))
	}

	out := make([]float32, len(in)/4)

	for i := 0; i < len(in); i += 4 {
		bits := binary.LittleEndian.Uint32(in[i : i+4])
		float := math.Float32frombits(bits)
		out[i/4] = float
	}

	return out, nil
}

func readVectorFromFile(name string) ([]float32, error) {
	// before := time.Now()
	// defer func() {
	// 	fmt.Printf("reading file took %s\n", time.Since(before))
	// }()
	f, err := os.Open(fmt.Sprintf("./data/%s", name))
	if err != nil {
		return nil, err
	}

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return vectorFromBytes(bytes)
}
