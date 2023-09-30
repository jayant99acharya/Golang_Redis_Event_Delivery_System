package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

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

// Mock function to simulate sending the event to a destination.
// Randomly returns success or failure.
func sendToDestination(event Event) bool {
	// Let's say there's an 80% chance of success.
	return rand.Intn(100) < 80
}

func processEvent() {
	for {
		// Pop an event from the front of Redis list (blocking until one is available).
		eventJSON, err := rdb.BLPop(ctx, 0*time.Second, "events").Result()
		if err != nil {
			log.Printf("Error fetching event from Redis: %v", err)
			time.Sleep(5 * time.Second) // Sleep for a while before retrying
			continue
		}

		var event Event
		if len(eventJSON) < 2 {
			log.Println("Error: Unexpected BLPop result format")
			continue
		}
		err = json.Unmarshal([]byte(eventJSON[1]), &event)
		if err != nil {
			log.Printf("Error unmarshaling event: %v", err)
			continue
		}

		success := sendToDestination(event)
		if success {
			log.Println("Event delivered successfully:", event)
		} else {
			log.Println("Failed to deliver event:", event)
		}
	}
}

func main() {
	initializeRedis()
	defer rdb.Close()

	// Start a Go routine for processing events.
	go processEvent()

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
