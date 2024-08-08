package main

import (
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"sync"
	"time"
)

type Log struct {
	Timestamp time.Time `json:"time" bson:"time"`
	Service   string    `json:"service" bson:"service"`
	Endpoint  string    `json:"endpoint" bson:"endpoint"`
	UserIp    string    `json:"user_ip" bson:"user_ip"`
	Success   bool      `json:"success" bson:"success"`
	Status    int       `json:"status" bson:"status"`
	Request   string    `json:"request" bson:"request"`
	Response  string    `json:"response" bson:"response"`
	Error     string    `json:"error" bson:"error"`
}

type Logger struct {
	collection *mongo.Collection
	logQueue   chan Log
	wg         sync.WaitGroup
}

func NewLogger(client *mongo.Client, dbName, collectionName string) *Logger {
	collection := client.Database(dbName).Collection(collectionName)
	l := &Logger{
		collection: collection,
		logQueue:   make(chan Log),
	}
	go l.processLogs()
	return l
}

func (l *Logger) Log(logEntry Log) {
	l.wg.Add(1)
	l.logQueue <- logEntry
	fmt.Println("Log eklendi")
}

func (l *Logger) processLogs() {
	defer l.wg.Done()
	for logEntry := range l.logQueue {
		_, err := l.collection.InsertOne(context.TODO(), logEntry)
		if err != nil {
			log.Printf("Error inserting log: %v", err)
		}
	}
}

func (l *Logger) Close() {
	close(l.logQueue)
	l.wg.Wait()
}

func NewLog(timestamp time.Time, success bool, service, error, endpoint, userIp, request, response string, status int) Log {
	return Log{
		Timestamp: timestamp,
		Service:   service,
		Endpoint:  endpoint,
		UserIp:    userIp,
		Success:   success,
		Status:    status,
		Request:   request,
		Response:  response,
		Error:     error,
	}
}
