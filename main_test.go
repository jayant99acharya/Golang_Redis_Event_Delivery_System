package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/go-redis/redis/v8"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Main", func() {
	var r *redis.Client

	BeforeEach(func() {
		r = setupRedisMock()
		rdb = r
	})

	AfterEach(func() {
		r.Close()
	})

	Describe("ingestEventHandler", func() {
		Context("with a valid POST request", func() {
			It("should add event to Redis", func() {
				mockEvent := Event{UserID: "testUser", Payload: "testPayload"}
				body, _ := json.Marshal(mockEvent)
				req, _ := http.NewRequest("POST", "/ingest", bytes.NewBuffer(body))
				rr := httptest.NewRecorder()

				handler := http.HandlerFunc(ingestEventHandler)
				handler.ServeHTTP(rr, req)

				Expect(rr.Code).To(Equal(http.StatusOK))
			})
		})
	})

	Describe("processEvent", func() {
		It("should process and remove the event from Redis", func() {
			mockEvent := Event{UserID: "testUser", Payload: "testPayload"}
			eventJSON, _ := json.Marshal(mockEvent)
			rdb.RPush(ctx, "events", eventJSON)

			processEvent()

			length := rdb.LLen(ctx, "events").Val()
			Expect(length).To(Equal(int64(0)))
		})
	})

	Describe("sendToDestination", func() {
		Context("with valid destinations", func() {
			It("should attempt to send to all destinations", func() {
				mockEvent := Event{UserID: "testUser", Payload: "testPayload"}
				success := sendToDestination(mockEvent)
				Expect(success).To(BeTrue())
			})
		})
	})

	Describe("processFailedEvents", func() {
		It("should process failed events and retry", func() {
			failedEvent := FailedEvent{Event: Event{UserID: "testUser", Payload: "testPayload"}, RetryCount: 0}
			eventJSON, _ := json.Marshal(failedEvent)
			rdb.ZAdd(ctx, "retry_events", &redis.Z{
				Score:  float64(time.Now().Unix()),
				Member: eventJSON,
			})

			processFailedEvents(1)

			retryLength := rdb.ZCard(ctx, "retry_events").Val()
			Expect(retryLength).To(Equal(int64(0)))
		})
	})
})

func setupRedisMock() *redis.Client {
	// Assuming a Redis test instance for this example.
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use a different DB for testing
	})
}

var _ = Describe("Integration", func() {
	var r *redis.Client

	BeforeEach(func() {
		r = setupRedisMock()
		rdb = r
	})

	AfterEach(func() {
		r.Close()
	})

	Context("from event ingestion to mock delivery", func() {
		It("should ingest, process, and deliver the event", func() {
			// Step 1: Ingest an event
			mockEvent := Event{UserID: "testUser", Payload: "testPayload"}
			body, _ := json.Marshal(mockEvent)
			req, _ := http.NewRequest("POST", "/ingest", bytes.NewBuffer(body))
			rr := httptest.NewRecorder()

			handler := http.HandlerFunc(ingestEventHandler)
			handler.ServeHTTP(rr, req)

			Expect(rr.Code).To(Equal(http.StatusOK))

			// Step 2: Process the event
			processEvent()

			// Step 3 & 4: Check event delivery or retry
			length := rdb.LLen(ctx, "events").Val()
			Expect(length).To(Equal(int64(0)))

			retryLength := rdb.ZCard(ctx, "retry_events").Val()
			Expect(retryLength).To(Equal(int64(0)))
		})
	})
})
