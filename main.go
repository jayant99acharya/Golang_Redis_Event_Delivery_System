package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Event represents the structure for the incoming event data
type Event struct {
	UserID  string `json:"userID"`
	Payload string `json:"payload"`
}

type FailedEvent struct {
	Event      Event
	RetryCount int
}

type Destination interface {
	Send(event Event) bool
}

var rdb *redis.Client
var ctx = context.Background()
var logger = logrus.New()

const (
	MaxRetries = 5
)

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
	destinations := []Destination{
		&MockDestination1{},
		&MockDestination2{},
		&MockDestination3{},
	}

	for _, dest := range destinations {
		success := dest.Send(event)
		if !success {
			return false // Event delivery failed for one of the destinations
		}
	}
	return true
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
			scheduleRetry(FailedEvent{
				Event:      event,
				RetryCount: 1, // Initial retry attempt
			})
		}
	}
}

func scheduleRetry(event FailedEvent) {
	// Increase the retry count
	event.RetryCount++

	// If retries are exhausted, log and potentially alert
	if event.RetryCount > MaxRetries {
		log.Printf("Failed to deliver event after %d attempts: %v", MaxRetries, event.Event)
		// TODO: Notify administrators or take other action
		return
	}

	// Calculate next retry time with exponential backoff
	backoffDuration := time.Duration(math.Pow(2, float64(event.RetryCount))) * time.Second
	retryTimestamp := time.Now().Add(backoffDuration).Unix()

	eventJSON, _ := json.Marshal(event)
	rdb.ZAdd(ctx, "retry_events", &redis.Z{
		Score:  float64(retryTimestamp),
		Member: eventJSON,
	})
}

func processFailedEvents(workerID int) {
	for {
		now := time.Now().Unix()

		// Fetch events scheduled for retry up to the current timestamp
		events, err := rdb.ZRangeByScoreWithScores(ctx, "retry_events", &redis.ZRangeBy{
			Min:    "0",
			Max:    fmt.Sprintf("%d", now),
			Offset: 0,
			Count:  1,
		}).Result()

		if err != nil {
			logger.WithFields(logrus.Fields{
				"workerID": workerID,
				"error":    err,
			}).Error("Failed to fetch events for retry")
			time.Sleep(5 * time.Second)
			continue
		}

		if len(events) == 0 {
			time.Sleep(5 * time.Second) // No events ready for retry, sleep for a while
			continue
		}

		var failedEvent FailedEvent
		err = json.Unmarshal([]byte(events[0].Member.(string)), &failedEvent)
		if err != nil {
			logger.WithFields(logrus.Fields{
				"workerID": workerID,
				"error":    err,
			}).Error("Error unmarshalling failed event")
			continue
		}

		success := sendToDestination(failedEvent.Event)
		if success {
			logger.WithFields(logrus.Fields{
				"workerID": workerID,
				"event":    failedEvent.Event,
			}).Info("Event delivered successfully on retry")

			rdb.ZRem(ctx, "retry_events", events[0].Member)
		} else {
			if failedEvent.RetryCount > MaxRetries {
				logger.WithFields(logrus.Fields{
					"workerID": workerID,
					"event":    failedEvent.Event,
				}).Warn("Max retries exhausted for event")
			} else {
				logger.WithFields(logrus.Fields{
					"workerID": workerID,
					"event":    failedEvent.Event,
				}).Info("Retry failed for event, rescheduling")
			}

			rdb.ZRem(ctx, "retry_events", events[0].Member)
			scheduleRetry(failedEvent)
		}
	}
}

// MockDestination1 : A destination that succeeds 80% of the time and fails 20% of the time
type MockDestination1 struct{}

func (md *MockDestination1) Send(event Event) bool {
	randNum := rand.Intn(100)
	if randNum < 80 {
		return true
	}
	return false
}

// MockDestination2 : A destination that introduces random delays (up to 2 seconds)
type MockDestination2 struct{}

func (md *MockDestination2) Send(event Event) bool {
	randDuration := time.Duration(rand.Intn(2000)) * time.Millisecond
	time.Sleep(randDuration)
	return true
}

// MockDestination3 : A destination that always succeeds but logs the received event
type MockDestination3 struct{}

func (md *MockDestination3) Send(event Event) bool {
	fmt.Printf("MockDestination3 received event: %+v\n", event)
	return true
}

func main() {
	initializeRedis()
	defer rdb.Close()

	// Start a Go routine for processing events.
	go processEvent()

	// Start a separate routine to process failed events
	for i := 0; i < 5; i++ {
		go processFailedEvents(i)
	}

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
