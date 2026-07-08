package main

import (
	"context"
	"fmt"
)

func main() {
	root := context.Background()

	node1 := context.WithValue(root, "A", 1)
	node2 := context.WithValue(node1, "B", 2)
	node3 := context.WithValue(node2, "C", 3)

	fmt.Println(node1.Value("A"), node2.Value("B"), node3.Value("C"), node3.Value("non-exist-key"))
}
