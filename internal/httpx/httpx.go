package httpx

import (
	"encoding/json"
	"net/http"
)

const (
	ErrInternalServer = "internal server error"
	ErrInvalidOrgID   = "invalid org id"
)

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
