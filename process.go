package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
	"log"
	"time"
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
