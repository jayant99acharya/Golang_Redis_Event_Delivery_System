package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
	"math"
	"time"
)

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
