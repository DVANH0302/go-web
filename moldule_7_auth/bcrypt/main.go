package main

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password []byte) ([]byte, error) {
	hashed, err := bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)

	return hashed, err
}

func CheckPassword(hashed, password []byte) error {
	err := bcrypt.CompareHashAndPassword(hashed, password)

	return err
}

func main() {
	pw := []byte("correct-horse-battery-staple")

	hashed, err := HashPassword(pw)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Hashed: %s\n", hashed)

	err = CheckPassword(hashed, pw)

	if err != nil {
		log.Fatal("error checking password: %v\n", err)
	}

	fmt.Println("Right password")

}
