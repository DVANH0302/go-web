package main

import (
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int `json:"user_id"`
	jwt.RegisteredClaims
}

var secretkey []byte = []byte("secret456")

func GenerateToken(userID int) (string, error) {

	// create the claim

	claim := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// create token with claim
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claim)

	// sign the token
	signed_token, err := token.SignedString(secretkey)

	return signed_token, err
}

func ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	// parse with claim -> *Token
	unverified_token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return secretkey, nil
	})

	if err != nil {
		log.Fatal("1", err)
	}

	if !unverified_token.Valid {
		return nil, err
	}

	return claims, nil
}

func main() {
	tokenString, err := GenerateToken(42)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(tokenString)

	// test token string signed by another secret key (secretkey123)
	// tokenString = "eyJhbGciOiJIUzUxMiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjo0MiwiZXhwIjoxNzgzNTA2NzMyLCJpYXQiOjE3ODM0MjAzMzJ9.sma0Vn55JFsNMEZ1wi6Hr7W2_0Eg7kurAcjWc562l_VDr7IP_cY3pvgwmpKPssTMYbazFgnKd8Mm-boo-vbs1Q"

	claims, err := ValidateToken(tokenString)

	if err != nil {
		log.Fatal("3", err)
	}

	fmt.Printf("\nClaims: %v\n", claims)
}
