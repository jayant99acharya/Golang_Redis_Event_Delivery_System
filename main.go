package main

import (
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"log"
	"net/http"
	"os"
)

var (
	rdb    RedisClientInterface   // Redis client
	ctx    = context.Background() // Global context for Redis operations
	logger = logrus.New()         // Logger instance
)

const (
	MaxRetries = 5 // Max attempts to retry sending an event

	adminEmail    = "jayant99acharya@gmail.com"
	emailServer   = "smtp.gmail.com:587"
	emailUser     = "jayant99acharya@gmail.com"
	emailPassword = "vtcb wdoy zloz hwhx"
)

func main() {
	initializeRedis() // Initialize our Redis client
	defer rdb.Close() // Ensure we close the Redis client when our program exits

	// Start a Go routine for processing events.
	go processEvent()

	// Start a separate routine to process failed events
	for i := 0; i < 5; i++ {
		go processFailedEvents(i)
	}

	// Set up our HTTP server.
	http.HandleFunc("/ingest", ingestEventHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Starting server on :%s...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
