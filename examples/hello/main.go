package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	greeting := os.Getenv("GREETING")
	if greeting == "" {
		greeting = "hello from zeroku"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		fmt.Fprintf(w, "%s (version 1)\n", greeting)
	})

	log.Printf("listening on :%s", port)

	log.Fatal(http.ListenAndServe(":"+port, nil))
}
