package handlers

import (
    "encoding/json"
    "net/http"
)

func SearchFighters(w http.ResponseWriter, r *http.Request) {
    // The middleware has already sanitized the name parameter
    name := r.URL.Query().Get("name")
    
    // Your search logic here - this is just an example
    w.Header().Set("Content-Type", "application/json")
    
    // Simple response for now
    response := map[string]interface{}{
        "search_term": name,
        "message": "Fighter search endpoint working",
    }
    
    json.NewEncoder(w).Encode(response)
}