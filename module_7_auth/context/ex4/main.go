package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func worker(ctx context.Context, thread_chan chan string, id int, wg *sync.WaitGroup) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("%d I am done\n", id)
			thread_chan <- fmt.Sprintf("Thread %d is Done\n", id)
			wg.Done()
			return
		default:
			fmt.Printf("%d working\n", id)
			time.Sleep(1 * time.Second)
		}
	}
}

func main() {
	parent, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	defer cancel()

	child1, cancel1 := context.WithCancel(parent)
	defer cancel1()

	child2, cancel2 := context.WithCancel(parent)
	defer cancel2()

	child3, cancel3 := context.WithCancel(parent)
	defer cancel3()

	thread_chan := make(chan string)

	var wg sync.WaitGroup
	wg.Add(3)

	go worker(child1, thread_chan, 1, &wg)
	go worker(child2, thread_chan, 2, &wg)
	go worker(child3, thread_chan, 3, &wg)

	go func() {
		wg.Wait()
		close(thread_chan)
	}()

	for thread_msg := range thread_chan {
		fmt.Printf("main received message from thread: %s", thread_msg)
		fmt.Println(parent.Err())
	}
}
