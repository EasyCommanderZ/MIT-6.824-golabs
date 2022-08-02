package main

import (
	"fmt"

	"golang.org/x/tour/tree"
)

// Walk 步进 tree t 将所有的值从 tree 发送到 channel ch。
func Walk(t *tree.Tree, ch chan int) {
	traverse(t, ch)
	close(ch)
}

func traverse(t *tree.Tree, ch chan int) {
	if t.Left != nil {
		traverse(t.Left, ch)
	}
	ch <- t.Value
	fmt.Println(t.Value)
	if t.Right != nil {
		traverse(t.Right, ch)
	}
}

// Same 检测树 t1 和 t2 是否含有相同的值。
func Same(t1, t2 *tree.Tree) bool {
	c1, c2 := make(chan int), make(chan int)
	go Walk(t1, c1)
	go Walk(t2, c2)
	for {
		v1, ok1 := <-c1
		v2, ok2 := <-c2
		if ok1 != ok2 || v1 != v2 {
			return false
		}
		if !ok1 || !ok2 {
			break
		}
	}
	return true
}

func main() {
	// Same(tree.New(1), tree.New(1))
	Walk(tree.New(1), make(chan int, 100))
}
