package main

import (
	"bytes"
	"encoding/json"
	"github.com/go-redis/redis/v8"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http"
	"net/http/httptest"
)

func setupRedisMock() RedisClientInterface {
	// Assuming a Redis test instance for this example.
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Used different DB for testing
	})
	return &RedisClientWrapper{Client: client}
}

var _ = Describe("Main", func() {

	BeforeEach(func() {
		rdb = setupRedisMock()
		rdb.Del(ctx, "events", "retry_events")
	})

	AfterEach(func() {
		rdb.Close()
		rdb.Del(ctx, "events", "retry_events")
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

	Describe("Integration", func() {
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
				processSingleEvent()

				// Step 3 & 4: Check event delivery or retry
				length := rdb.LLen(ctx, "events").Val()
				Expect(length).To(Equal(int64(0)))

				retryLength := rdb.ZCard(ctx, "retry_events").Val()
				Expect(retryLength).To(Equal(int64(0)))
			})
		})
	})
})
