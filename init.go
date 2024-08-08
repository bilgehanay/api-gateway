package main

import (
	"context"
	"encoding/json"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"os"
)

var (
	config ConfigModel
	L      *Logger
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
	mongoConn := options.Client().ApplyURI("mongodb://localhost:27017")
	mongoConn.SetAppName("logger")
	mc, err := mongo.Connect(context.TODO(), mongoConn)
	if err != nil {
		panic(err)
	}

	L = NewLogger(mc, "logger", "logger")
}
