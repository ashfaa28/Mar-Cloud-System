package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var jwtKey = []byte("rahasia-super-aman")

// hardcoded user buat contoh
var users = map[string]string{
	"user1": "pass1",
}

// Struktur klaim
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// EndPoint Login
func loginHandler(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	if pass, ok := users[username]; !ok || pass != password {
		http.Error(w, "Username atau Password Salah!!", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(1 * time.Hour)
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		http.Error(w, "Gagal Membuat Token", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, tokenString)
}

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tokenStr := r.Header.Get("Authorization")
		if tokenStr == "" || len(tokenStr) < 8 || tokenStr[:7] != "Bearer " {
			http.Error(w, "Token Tidak Valid", http.StatusUnauthorized)
			return
		}

		tokenStr = tokenStr[7:] // buang "Bearer "

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Token expired atau tidak sah", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}
