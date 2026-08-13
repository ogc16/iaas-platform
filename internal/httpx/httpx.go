package httpx

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
)

const (
	ErrInternalServer = "internal server error"
	ErrInvalidOrgID   = "invalid org id"
)

func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// encoding/json marshals a nil slice as null. List endpoints should return
	// [] so browser clients can safely call Array methods.
	if data != nil {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Slice && v.IsNil() {
			data = reflect.MakeSlice(v.Type(), 0, 0).Interface()
		}
	}
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
