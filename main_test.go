package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

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
})

func setupRedisMock() *redis.Client {
	// Assuming a Redis test instance for this example.
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use a different DB for testing
	})
}
