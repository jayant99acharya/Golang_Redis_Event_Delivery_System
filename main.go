package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/go-redis/redis/v8"
	"golang.org/x/net/context"
)

// Event represents the structure for the incoming event data
type Event struct {
	UserID  string `json:"userID"`
	Payload string `json:"payload"`
}

var rdb *redis.Client
var ctx = context.Background()

func initializeRedis() {
	rdb = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No password
		DB:       0,                // Default DB
	})

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Error initializing Redis: %v", err)
	}
}

func ingestEventHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is supported", http.StatusMethodNotAllowed)
		return
	}

	var event Event
	err := json.NewDecoder(r.Body).Decode(&event)
	if err != nil {
		http.Error(w, "Error parsing event", http.StatusBadRequest)
		return
	}

	eventJSON, _ := json.Marshal(event)
	if err := rdb.RPush(ctx, "events", eventJSON).Err(); err != nil {
		http.Error(w, "Error saving event to Redis", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "Event ingested successfully")
}

func main() {
	initializeRedis()
	defer rdb.Close()

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
