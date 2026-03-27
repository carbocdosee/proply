package handler

import (
	"encoding/json"
	"net/http"
)

// respond writes a JSON response with the given status code and body.
func respond(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, code string) {
	respond(w, status, map[string]string{"code": code})
}

// respondOK writes a 200 JSON response.
func respondOK(w http.ResponseWriter, body any) {
	respond(w, http.StatusOK, body)
}
