package main

import (
	"encoding/json"
	"log"
	"os"
)

var (
	config ConfigModel
)

func init() {
	file, err := os.Open("endpoints.json")
	if err != nil {
		log.Fatalf("Failed to open config file: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatalf("Failed to parse config file: %v", err)
	}
}
