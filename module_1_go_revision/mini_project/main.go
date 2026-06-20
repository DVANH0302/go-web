package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func main() {
	url := os.Args[1]
	fmt.Printf("%s \n", url)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Printf("Content-Type: %s", resp.Header.Get("Content-Type"))

	body_limit := make([]byte, 200)
	for i := 0; i < 200; i++ {
		body_limit[i] = body[i]
	}

	fmt.Printf("Body preview:\n%s", body_limit)
}
