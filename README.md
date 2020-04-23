# knn-nws (golang) - WIP!
> POC Golang KNN: Navigable Small Worlds Graph Approximate KNN

A simple POC to better understand the [Navigable Small World Graph
Algorithm](https://www.sciencedirect.com/science/article/abs/pii/S0306437913001300?via%3Dihub)
to approximate k-Nearest-Neighbors calculation. 

## How to run

Simply do a `go run .` which builds a tree based on the vectors in
`./vectors.txt`. Everything is hard-coded, but you can adjust the `main()`
function if you really want to.

## Roadmap

* [x] simple POC without binary search trees ("TreeSet")
  * works well, but isn't the most efficient without BSTs
* [ ] build binary search trees
  * [x] everything but delete
  * [ ] delete
* [ ] validate algo is (much) faster with BSTs

## Code Quality level
Quick and dirty POC / Spike
