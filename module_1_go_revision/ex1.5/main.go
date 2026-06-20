package main

import (
	"fmt"
	"math/rand"
	"time"
)

/*
Write a program that launches 5 goroutines,
each sleeping for a random duration between 100ms and 500ms,
then sending their goroutine number on a channel.
The main function should collect all 5 results
and print them in the order they arrive.
*/

func main() {
	ch := make(chan int)
	for i := 0; i < 5; i++ {
		go func(id int) {
			myRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			time.Sleep(time.Duration(myRand.Intn(3)) * time.Second)
			ch <- id
		}(i)
	}

	for i := 0; i < 5; i++ {
		msg := <-ch
		fmt.Println(msg)
	}
}
