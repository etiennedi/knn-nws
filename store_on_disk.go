package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"syscall"
	"time"

	"github.com/boltdb/bolt"
)

const vectorDimensions = 600
const vectorSize = 4 // float32

var magicMappedFile []byte

func initMagicMappedFile() {
	path := "./data/vectors"
	file, err := os.Open(path)
	if err != nil {
		log.Fatalf("Can't open the knn file at %s: %+v", path, err)
	}

	file_info, err := file.Stat()
	if err != nil {
		log.Fatalf("Can't stat the knn file at %s: %+v", path, err)
	}

	mmap, err := syscall.Mmap(int(file.Fd()), 0, int(file_info.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		log.Fatalf("Can't mmap the knn file %s: %+v", path, err)
	}

	magicMappedFile = mmap
}

func storeToFile(index int64, vector []float32) error {
	f, err := os.OpenFile("./data/vectors", os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Seek(index*vectorDimensions*vectorSize, 0)
	_, err = f.Write(vectorToBytes(vector))
	if err != nil {
		return err
	}

	return nil
}

func storeToBolt(index int64, vector []float32) error {
	before := time.Now()
	defer m.addWritingDisk(before)

	err := db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Vectors"))
		err := b.Put([]byte(fmt.Sprintf("%d", index)), vectorToBytes(vector))
		return err
	})

	if err != nil {
		return fmt.Errorf("store to bolt: %v", err)
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

func readVectorFromBolt(i int64) ([]float32, error) {
	before := time.Now()
	defer m.addReadingDisk(before)

	var out []float32
	var err error
	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("Vectors"))
		v := b.Get([]byte(fmt.Sprintf("%d", i)))
		out, err = vectorFromBytes(v)
		return nil
	})

	return out, err
}

func readVectorFromFile(i int64) ([]float32, error) {
	before := time.Now()
	defer m.addReadingDisk(before)

	start := i * vectorDimensions * vectorSize
	end := start + vectorDimensions*vectorSize
	bytes := magicMappedFile[start:end]

	return vectorFromBytes(bytes)
}
