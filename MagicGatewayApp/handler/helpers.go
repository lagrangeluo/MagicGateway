package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"runtime/debug"
	"strconv"
	"time"
)

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func errResp(msg string) map[string]string {
	return map[string]string{"error": msg}
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[recover] panic: %v\n%s", rec, debug.Stack())
				writeJSON(w, 500, errResp("internal server error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
