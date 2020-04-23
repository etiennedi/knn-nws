package main

import "fmt"

type binarySearchTree struct {
	root *binarySearchNode
}

func (t *binarySearchTree) insert(data *vertex, dist float32) {
	if t.root == nil {
		t.root = &binarySearchNode{
			data: data,
			dist: dist,
		}
		return
	}

	t.root.insert(data, dist)
}

func (t *binarySearchTree) printInOrder() {
	t.root.printInOrder()
}

func (t *binarySearchTree) contains(data *vertex, dist float32) bool {
	if t.root == nil {
		return false
	}

	return t.root.contains(data, dist)
}

func (t *binarySearchTree) minimum() *binarySearchNode {
	return t.root.minimum()
}

func (t *binarySearchTree) flattenInOrder() []*binarySearchNode {
	return t.root.flattenInOrder()
}

type binarySearchNode struct {
	data  *vertex
	dist  float32
	left  *binarySearchNode
	right *binarySearchNode
}

func (n *binarySearchNode) insert(data *vertex, dist float32) {
	if n == nil {
		n = &binarySearchNode{
			data: data,
			dist: dist,
		}
	}

	if dist == n.dist && data == n.data {
		// exact node is already present, ignore
		return
	}

	if dist < n.dist {
		if n.left != nil {
			n.left.insert(data, dist)
			return
		} else {
			n.left = &binarySearchNode{
				data: data,
				dist: dist,
			}
			return
		}
	} else {
		if n.right != nil {
			n.right.insert(data, dist)
			return
		} else {
			n.right = &binarySearchNode{
				data: data,
				dist: dist,
			}
			return
		}
	}
}

func (n *binarySearchNode) printInOrder() {
	if n == nil {
		return
	}

	if n.left != nil {
		n.left.printInOrder()
	}

	fmt.Printf("%f - %v\n", n.dist, n.data)

	if n.right != nil {
		n.right.printInOrder()
	}
}

func (n *binarySearchNode) contains(data *vertex, dist float32) bool {
	if n.data == data {
		return true
	}

	if dist < n.dist {
		if n.left == nil {
			return false
		}

		return n.left.contains(data, dist)
	} else {
		if n.right == nil {
			return false
		}

		return n.right.contains(data, dist)

	}
}

func (n *binarySearchNode) minimum() *binarySearchNode {
	if n.left == nil {
		return n
	}

	return n.left.minimum()
}

func (n *binarySearchNode) flattenInOrder() []*binarySearchNode {
	var left []*binarySearchNode
	var right []*binarySearchNode

	if n.left != nil {
		left = n.left.flattenInOrder()
	}

	if n.right != nil {
		right = n.right.flattenInOrder()
	}

	right = append([]*binarySearchNode{n}, right...)
	return append(left, right...)
}
