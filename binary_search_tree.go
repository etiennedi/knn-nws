package main

import (
	"fmt"
	"time"
)

type binarySearchTree struct {
	root *binarySearchNode
}

func (t *binarySearchTree) insert(data *vertex, dist float32) {
	before := time.Now()
	if t.root == nil {
		t.root = &binarySearchNode{
			data: data,
			dist: dist,
		}
		return
	}

	t.root.insert(data, dist)
	spentInserting += time.Since(before)
}

func (t *binarySearchTree) printInOrder() {
	t.root.printInOrder()
}

func (t *binarySearchTree) contains(data *vertex, dist float32) bool {
	before := time.Now()
	defer func() {
		spentContains += time.Since(before)
	}()

	if t.root == nil {
		return false
	}

	return t.root.contains(data, dist)
}

func (t *binarySearchTree) minimum() *binarySearchNode {
	return t.root.minimum()
}

func (t *binarySearchTree) flattenInOrder() []*binarySearchNode {
	before := time.Now()
	defer func() {
		spentFlattening += time.Since(before)
	}()

	if t.root == nil {
		return nil
	}

	return t.root.flattenInOrder()
}

func (t *binarySearchTree) delete(data *vertex, dist float32) {
	before := time.Now()

	fakeParent := &binarySearchNode{right: t.root, data: &vertex{object: "fake node"}, dist: -999999}

	t.root.delete(data, dist, fakeParent)
	t.root = fakeParent.right
	spentDeleting += time.Since(before)
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

// maxAndParent() is a helper function for swapping while deleting
func (n *binarySearchNode) maxAndParent(parent *binarySearchNode) (*binarySearchNode, *binarySearchNode) {
	if n == nil {
		return nil, parent
	}

	if n.right == nil {
		return n, parent
	}

	return n.right.maxAndParent(n)
}

// minAndParent() is a helper function for swapping while deleting
func (n *binarySearchNode) minAndParent(parent *binarySearchNode) (*binarySearchNode, *binarySearchNode) {
	if n == nil {
		return nil, parent
	}

	if n.left == nil {
		return n, parent
	}

	return n.left.minAndParent(n)
}

func (n *binarySearchNode) replaceNode(parent, replacement *binarySearchNode) {
	if n == nil {
		panic("tried tor replace nil node")
	}

	if n == parent.left {
		// the current node is the parent's left, so we replace our parent's left
		// node, i.e. ourself
		parent.left = replacement
	} else {
		// vice versa for right
		parent.right = replacement
		if replacement != nil {
		}
	}
}

// delete is inspired by the great explanation at https://appliedgo.net/bintree/
func (n *binarySearchNode) delete(data *vertex, dist float32, parent *binarySearchNode) {
	if n == nil {
		panic(fmt.Sprintf("trying to delete nil node %v of parent %v", data, parent))
	}

	if n.dist == dist && n.data == data {
		// this is the node to be deleted

		if n.left == nil && n.right == nil {
			// node has no children, so deletion is a simple as removing this node
			n.replaceNode(parent, nil)
			return
		}

		// if the node has just one child, simply swap with it's child
		if n.left == nil {

			n.replaceNode(parent, n.right)
			return
		}
		if n.right == nil {
			n.replaceNode(parent, n.left)
			return
		}

		// node has two children
		if parent.right != nil && parent.right.data == n.data {
			// this node is a right child, so we need to delete max from left
			replacement, replParent := n.left.maxAndParent(n)
			n.data = replacement.data
			n.dist = replacement.dist

			replacement.delete(replacement.data, replacement.dist, replParent)
			return
		}

		if parent.left != nil && parent.left.data == n.data {
			// this node is a left child, so we need to delete min from right
			replacement, replParent := n.right.minAndParent(n)
			n.data = replacement.data
			n.dist = replacement.dist

			replacement.delete(replacement.data, replacement.dist, replParent)
			return
		}

		panic("this should be unreachable")
	}

	if dist < n.dist {
		n.left.delete(data, dist, n)
		return
	} else {
		n.right.delete(data, dist, n)
	}

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
