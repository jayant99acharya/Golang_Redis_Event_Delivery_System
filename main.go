package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

var (
	rdb    RedisClientInterface   // Redis client
	ctx    = context.Background() // Global context for Redis operations
	logger = logrus.New()         // Logger instance
)

const (
	MaxRetries = 5 // Max attempts to retry sending an event

	adminEmail    = "admin@example.com"
	emailServer   = "smtp.example.com:587"
	emailUser     = "notify@example.com"
	emailPassword = "password"
)

// Event represents the structure for the incoming event data
type Event struct {
	UserID  string `json:"userID"`
	Payload string `json:"payload"`
}

// FailedEvent represents an event that has failed delivery, along with the count of retry attempts.
type FailedEvent struct {
	Event      Event
	RetryCount int
}

// Destination interface represents any target to which we want to send our event.
type Destination interface {
	Send(event Event) bool
}

// RedisClientInterface interface represents the command used by redis client
type RedisClientInterface interface {
	Ping(ctx context.Context) *redis.StatusCmd
	RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd
	LLen(ctx context.Context, key string) *redis.IntCmd
	ZAdd(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd
	ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd
	ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd
	BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd
	ZCard(ctx context.Context, key string) *redis.IntCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	Close() error
}

// RedisClientWrapper that wraps the actual client, satisfying our interface
type RedisClientWrapper struct {
	Client *redis.Client
}

func (wrapper *RedisClientWrapper) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return wrapper.Client.Del(ctx, keys...)
}

func (wrapper *RedisClientWrapper) ZCard(ctx context.Context, key string) *redis.IntCmd {
	return wrapper.Client.ZCard(ctx, key)
}

func (wrapper *RedisClientWrapper) BLPop(ctx context.Context, timeout time.Duration, keys ...string) *redis.StringSliceCmd {
	return wrapper.Client.BLPop(ctx, timeout, keys...)
}

func (wrapper *RedisClientWrapper) Close() error {
	return wrapper.Client.Close()
}

func (wrapper *RedisClientWrapper) Ping(ctx context.Context) *redis.StatusCmd {
	return wrapper.Client.Ping(ctx)
}

func (wrapper *RedisClientWrapper) RPush(ctx context.Context, key string, values ...interface{}) *redis.IntCmd {
	return wrapper.Client.RPush(ctx, key, values...)
}

func (wrapper *RedisClientWrapper) LLen(ctx context.Context, key string) *redis.IntCmd {
	return wrapper.Client.LLen(ctx, key)
}

func (wrapper *RedisClientWrapper) ZAdd(ctx context.Context, key string, members ...*redis.Z) *redis.IntCmd {
	return wrapper.Client.ZAdd(ctx, key, members...)
}

func (wrapper *RedisClientWrapper) ZRangeByScoreWithScores(ctx context.Context, key string, opt *redis.ZRangeBy) *redis.ZSliceCmd {
	return wrapper.Client.ZRangeByScoreWithScores(ctx, key, opt)
}

func (wrapper *RedisClientWrapper) ZRem(ctx context.Context, key string, members ...interface{}) *redis.IntCmd {
	return wrapper.Client.ZRem(ctx, key, members...)
}

// initializeRedis sets up a connection to the Redis server.
func initializeRedis() {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Redis server address
		Password: "",               // No password
		DB:       0,                // Default DB
	})

	rdb = &RedisClientWrapper{Client: client}

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Error initializing Redis: %v", err)
	}
}

// ingestEventHandler handles incoming HTTP requests to ingest events.
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

// processEvent continuously tries to fetch events from Redis and sends them to their destinations.
func processEvent() {
	for {
		processSingleEvent()
	}
}

// processSingleEvent tries to fetch one event from Redis and sends it to its destination.
func processSingleEvent() {
	// Pop an event from the front of Redis list (blocking until one is available).
	eventJSON, err := rdb.BLPop(ctx, 0*time.Second, "events").Result()
	if err != nil {
		log.Printf("Error fetching event from Redis: %v", err)
		return
	}

	if len(eventJSON) < 2 {
		log.Println("Error: Unexpected BLPop result format")
		return
	}
	var event Event
	err = json.Unmarshal([]byte(eventJSON[1]), &event)
	if err != nil {
		log.Printf("Error unmarshaling event: %v", err)
		return
	}

	success := sendToDestination(event)
	if !success {
		log.Println("Failed to deliver event:", event)
		scheduleRetry(FailedEvent{
			Event:      event,
			RetryCount: 1, // Initial retry attempt
		})
	}
}

// scheduleRetry schedules the failed event for a retry in the future.
func scheduleRetry(event FailedEvent) {
	// Increase the retry count
	event.RetryCount++

	// If retries are exhausted, log and potentially alert
	if event.RetryCount > MaxRetries {
		log.Printf("Failed to deliver event after %d attempts: %v", MaxRetries, event.Event)
		notifyAdmin("Event Delivery Failed", fmt.Sprintf("Failed to deliver event after %d attempts: %v", MaxRetries, event.Event))
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

// processFailedEvents continuously checks for events that are due for a retry and attempts to send them.
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

// notifyAdmin sends an email notification to the admin about the failure to deliver an event.
func notifyAdmin(subject, body string) {
	from := emailUser
	to := []string{adminEmail}
	msg := "From: " + from + "\n" +
		"To: " + adminEmail + "\n" +
		"Subject: " + subject + "\n\n" +
		body

	err := smtp.SendMail(emailServer, smtp.PlainAuth("", emailUser, emailPassword, "smtp.example.com"), from, to, []byte(msg))
	if err != nil {
		log.Printf("Error notifying admin: %v", err)
	}
}

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
