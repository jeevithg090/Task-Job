package main

import (
	"fmt"
	"log"
	"net/http"

	"backend-auth/database"
	"backend-auth/handlers"
	"backend-auth/middleware"
)

// cors wraps a handler to add CORS headers
func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next(w, r)
	}
}

func main() {
	// Initialize SQLite database
	database.InitDB()

	// Serve static frontend files
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// Register API routes with CORS
	http.HandleFunc("/signup", cors(handlers.Signup))
	http.HandleFunc("/login", cors(handlers.Login))
	http.HandleFunc("/profile", cors(middleware.JWTAuth(handlers.Profile)))
	http.HandleFunc("/users", cors(middleware.JWTAuth(handlers.GetUsers)))

	// Start server
	fmt.Println("Server running on port 8080")
	fmt.Println("Frontend: http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
