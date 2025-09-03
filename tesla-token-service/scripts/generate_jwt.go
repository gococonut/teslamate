package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	var (
		secret   = flag.String("secret", "", "JWT secret key")
		subject  = flag.String("subject", "teslamate", "JWT subject")
		duration = flag.String("duration", "8760h", "Token duration (e.g., 24h, 7d, 8760h)")
	)
	flag.Parse()

	if *secret == "" {
		log.Fatal("Secret key is required. Use -secret flag.")
	}

	// 解析持续时间
	dur, err := time.ParseDuration(*duration)
	if err != nil {
		log.Fatalf("Invalid duration format: %v", err)
	}

	// 创建 JWT claims
	claims := jwt.RegisteredClaims{
		Subject:   *subject,
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(dur)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
		Issuer:    "tesla-token-service",
	}

	// 创建 token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(*secret))
	if err != nil {
		log.Fatalf("Failed to generate token: %v", err)
	}

	fmt.Printf("Generated JWT Token:\n%s\n\n", tokenString)
	fmt.Printf("Token Details:\n")
	fmt.Printf("Subject: %s\n", *subject)
	fmt.Printf("Expires: %s\n", claims.ExpiresAt.Time.Format(time.RFC3339))
	fmt.Printf("Duration: %s\n", dur.String())
	fmt.Printf("\nUse this token in the Authorization header as:\n")
	fmt.Printf("Authorization: Bearer %s\n", tokenString)
}