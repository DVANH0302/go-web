package main

import "fmt"

/*
Drill 1.2 — Zero values
Declare a User struct with Name string, Age int, Active bool.
Create one without assigning any fields.
Print each field and observe the zero values.
Then write a method IsComplete() bool
that returns true only if Name is non-empty and Age is greater than 0.
*/

type User struct {
	Name   string
	Age    int
	Active bool
}

func (u *User) IsComplete() bool {
	if u.Name == "" {
		fmt.Printf("Error, name is emtpy\n")
		return false
	}
	if u.Age <= 0 {
		fmt.Printf("Error: age is empty\n")
		return false
	}

	return true
}

func main() {
	var u *User = &User{}
	fmt.Printf("%s, %d, %v\n", u.Name, u.Age, u.Active)

	r := u.IsComplete()

	fmt.Printf("%v\n", r)
}
