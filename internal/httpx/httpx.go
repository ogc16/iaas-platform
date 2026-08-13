package httpx

import (
	"encoding/json"
	"errors"
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

// WriteError maps a sentinel error to its status code via the statuses map.
// Unmatched errors are reported as internal server errors.
func WriteError(w http.ResponseWriter, err error, statuses map[error]int) {
	for sentinel, status := range statuses {
		if errors.Is(err, sentinel) {
			WriteJSON(w, status, map[string]string{"error": err.Error()})
			return
		}
	}
	WriteJSON(w, http.StatusInternalServerError, map[string]string{"error": ErrInternalServer})
}
