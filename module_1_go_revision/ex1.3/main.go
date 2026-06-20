package main

import (
	"fmt"
	"strconv"
)

/*
Drill 1.3 — Errors as values
Write a function parseAge(s string) (int, error) that:

Uses strconv.Atoi to convert a string to int
Returns an error if the conversion fails
Returns an error if the age is less than 0 or greater than 150
Returns the age and nil otherwise
Call it with "25", "abc", "-5", and "200" and handle each error appropriately.
*/

func parseAge(s string) (int, error) {
	r, err := strconv.Atoi(s)

	return r, err
}

func main() {
	s := "-2dsfa5"
	r, err := parseAge(s)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("%d\n", r)
}
