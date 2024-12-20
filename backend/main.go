// main.go
package main

import (
    "net/http"
    "fmt"
)

func main() {
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintln(w, "Hello, world!")
    })
    
    fmt.Println("Server is running on port 8080")
    http.ListenAndServe(":8080", nil)
}
