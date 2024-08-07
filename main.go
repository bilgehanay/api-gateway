package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fmt.Println("API Gateway running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", Router()))
}
