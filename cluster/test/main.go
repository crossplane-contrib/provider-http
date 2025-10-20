package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

const SecretKey = "my-secret-value"

// In-memory data store
var dataStore = make(map[string]map[string]interface{})

// User represents a user object
type User map[string]interface{}

// Response represents an API response
type Response struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token string `json:"token"`
}

// NotifyResponse represents the notify response
type NotifyResponse struct {
	ID     int                    `json:"id"`
	Status string                 `json:"status"`
	Test   map[string]interface{} `json:"test"`
}

// Authorization middleware
func requireAuthorization(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		var token string
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if strings.HasPrefix(authHeader, "Basic ") {
			token = strings.TrimPrefix(authHeader, "Basic ")
		} else {
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if token != SecretKey {
			writeErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// Custom header middleware
func addCustomHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Secret-Header", "YourSecretValue")
		next.ServeHTTP(w, r)
	})
}

// Helper function to write JSON responses
func writeJSONResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// Helper function to write error responses
func writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	writeJSONResponse(w, Response{Error: message}, statusCode)
}

// GET /v1/users/{user_id}
func getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	user, exists := dataStore[userID]
	if !exists {
		writeErrorResponse(w, "User not found", http.StatusNotFound)
		return
	}

	fmt.Println(user) // Equivalent to Python's print(user)
	writeJSONResponse(w, user, http.StatusOK)
}

// POST /v1/users
func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		writeErrorResponse(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	userID := strconv.Itoa(len(dataStore))
	user["id"] = userID
	dataStore[userID] = user

	writeJSONResponse(w, user, http.StatusCreated)
}

// PUT /v1/users/{user_id}
func updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	existingUser, exists := dataStore[userID]
	if !exists {
		writeErrorResponse(w, "User not found", http.StatusNotFound)
		return
	}

	var userUpdates User
	if err := json.NewDecoder(r.Body).Decode(&userUpdates); err != nil {
		writeErrorResponse(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Update the existing user with new fields
	for key, value := range userUpdates {
		existingUser[key] = value
	}

	writeJSONResponse(w, existingUser, http.StatusOK)
}

// DELETE /v1/users/{user_id}
func deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	if _, exists := dataStore[userID]; !exists {
		writeErrorResponse(w, "user_id not found", http.StatusNotFound)
		return
	}

	delete(dataStore, userID)
	writeJSONResponse(w, Response{Message: "User deleted"}, http.StatusOK)
}

// POST /v1/login
func obtainToken(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, LoginResponse{Token: SecretKey}, http.StatusCreated)
}

// POST /v1/notify
func notify(w http.ResponseWriter, r *http.Request) {
	response := NotifyResponse{
		ID:     123,
		Status: "sent",
		Test: map[string]interface{}{
			"one": "one",
			"two": "two",
			"nested": map[string]interface{}{
				"try": "try",
			},
		},
	}
	writeJSONResponse(w, response, http.StatusCreated)
}

func main() {
	r := mux.NewRouter()

	// Apply custom header middleware to all routes
	r.Use(addCustomHeader)

	// User routes with authorization
	r.HandleFunc("/v1/users/{user_id}", requireAuthorization(getUser)).Methods("GET")
	r.HandleFunc("/v1/users", requireAuthorization(createUser)).Methods("POST")
	r.HandleFunc("/v1/users/{user_id}", requireAuthorization(updateUser)).Methods("PUT")
	r.HandleFunc("/v1/users/{user_id}", requireAuthorization(deleteUser)).Methods("DELETE")

	// Login route with authorization
	r.HandleFunc("/v1/login", requireAuthorization(obtainToken)).Methods("POST")

	// Notify route with authorization
	r.HandleFunc("/v1/notify", requireAuthorization(notify)).Methods("POST")

	fmt.Println("Server starting on :5000...")
	log.Fatal(http.ListenAndServe(":5000", r))
}
