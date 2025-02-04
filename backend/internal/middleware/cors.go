package middleware

import (
	"net/http"
	"strconv"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string
	AllowedMethods []string
	AllowedHeaders []string
	MaxAge         int
}

func CORS(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if len(config.AllowedOrigins) == 0 {
				config.AllowedOrigins = []string{"*"}
			}
			if len(config.AllowedMethods) == 0 {
				config.AllowedMethods = []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}
			}
			if len(config.AllowedHeaders) == 0 {
				config.AllowedHeaders = []string{
					"Accept",
					"Authorization",
					"Content-Type",
					"X-CSRF-Token",
					"X-Requested-With",
				}
			}

			w.Header().Set("Access-Control-Allow-Origin", strings.Join(config.AllowedOrigins, ","))
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ","))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ","))

			if config.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
			}

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
