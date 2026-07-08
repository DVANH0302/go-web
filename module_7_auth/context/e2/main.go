package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	thread_chan := make(chan string, 1)

	go func(ctx context.Context, thread_chan chan string) {
		for {
			select {
			case <-ctx.Done():
				fmt.Println("I am done")
				thread_chan <- "Thread is Done\n"
				return
			default:
				fmt.Println("I am doing the work")
				time.Sleep(1 * time.Second)

			}
		}

	}(ctx, thread_chan)

	time.Sleep(2 * time.Second)

	cancel()

	thread_msg := <-thread_chan

	fmt.Printf("main received message from thread: %s", thread_msg)
	fmt.Println(ctx.Err())
}
