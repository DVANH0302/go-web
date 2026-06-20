package main

import "fmt"

/*
Create a package-level struct Config with three fields:
one exported string, one unexported int, one exported bool.
Write an exported constructor function NewConfig
that returns a *Config with some defaults set.
*/

type Config struct {
	Email    string
	key      int
	IsSecure bool
}

func NewConfig() *Config {
	return &Config{
		Email:    "abc@gmail.com",
		key:      123,
		IsSecure: true,
	}
}

func main() {
	var a *Config = NewConfig()

	fmt.Printf("%s, %d, %v\n", a.Email, a.key, a.IsSecure)
}
