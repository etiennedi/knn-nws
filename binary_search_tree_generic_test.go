package main

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestTree(t *testing.T) {

	bst := &binarySearchTreeGeneric{}

	for i := 0; i < 100000; i++ {
		num := rand.Intn(100000)
		bst.insert(num, float32(num))
	}

	fmt.Printf("%v", bst.flattenInOrder())

	for bst.len() > 0 {
		min := bst.minimum()

		bst.delete(min.index, min.dist)
		fmt.Printf("%v", bst.flattenInOrder())

	}

	t.Fail()

}
