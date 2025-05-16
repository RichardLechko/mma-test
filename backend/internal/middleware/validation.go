package middleware

import (
    "net/http"
    "regexp"
    "strings"
)

func sanitizeFighterName(name string) string {
    // Remove dangerous characters
    name = regexp.MustCompile(`[<>"'%;()&+]`).ReplaceAllString(name, "")
    // Limit length
    if len(name) > 100 {
        name = name[:100]
    }
    return strings.TrimSpace(name)
}

func ValidateFighterSearch(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Get the search parameter from URL query
        name := r.URL.Query().Get("name")
        
        // Sanitize the fighter name input
        sanitized := sanitizeFighterName(name)
        
        // Update the request URL with sanitized value
        q := r.URL.Query()
        q.Set("name", sanitized)
        r.URL.RawQuery = q.Encode()
        
        next.ServeHTTP(w, r)
    })
}