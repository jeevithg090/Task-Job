package handlers

import (
	"encoding/json"
	"net/http"

	"backend-auth/database"
	"backend-auth/middleware"
	"backend-auth/models"

	"github.com/golang-jwt/jwt/v5"
)

// Profile returns the logged-in user's profile
func Profile(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Extract claims from request context (set by JWT middleware)
	claims, ok := r.Context().Value(middleware.ClaimsKey).(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}

	// Extract user ID from claims
	userID := int(claims["user_id"].(float64))

	// Fetch user from database
	var user models.User
	err := database.DB.QueryRow(
		"SELECT id, name, email, role FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Name, &user.Email, &user.Role)
	if err != nil {
		http.Error(w, `{"error":"User not found"}`, http.StatusNotFound)
		return
	}

	// Return user profile
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// GetUsers returns all users (admin only)
func GetUsers(w http.ResponseWriter, r *http.Request) {
	// Only allow GET method
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// Extract claims from request context
	claims, ok := r.Context().Value(middleware.ClaimsKey).(jwt.MapClaims)
	if !ok {
		http.Error(w, `{"error":"Invalid token claims"}`, http.StatusUnauthorized)
		return
	}

	// Check if user has admin role
	if claims["role"] != "admin" {
		http.Error(w, `{"error":"Forbidden: admin access required"}`, http.StatusForbidden)
		return
	}

	// Query all users from database
	rows, err := database.DB.Query("SELECT id, name, email, role FROM users")
	if err != nil {
		http.Error(w, `{"error":"Database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Build users list
	var users []models.User
	for rows.Next() {
		var user models.User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.Role); err != nil {
			http.Error(w, `{"error":"Failed to scan user"}`, http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}

	// Return all users
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
