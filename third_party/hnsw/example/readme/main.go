package main

import (
	"fmt"
	"log"

	"github.com/coder/hnsw"
)

func main() {
	g := hnsw.NewGraph[int]()
	if err := g.Add(
		hnsw.MakeNode(1, []float32{1, 1, 1}),
		hnsw.MakeNode(2, []float32{1, -1, 0.999}),
		hnsw.MakeNode(3, []float32{1, 0, -0.5}),
	); err != nil {
		log.Fatal(err)
	}

	neighbors, err := g.Search(
		[]float32{0.5, 0.5, 0.5},
		1,
	)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("best friend: %v\n", neighbors[0].Value)
}
