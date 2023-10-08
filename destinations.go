package main

import (
	"fmt"
	"math/rand"
	"time"
)

// Destination interface represents any target to which we want to send our event.
type Destination interface {
	Send(event Event) bool
}

// MockDestination1 : A destination that succeeds 80% of the time and fails 20% of the time
type MockDestination1 struct{}

func (md *MockDestination1) Send(event Event) bool {
	randNum := rand.Intn(100)
	if randNum < 80 {
		fmt.Printf("MockDestination1 successfully received event: %+v\n", event)
		return true
	}
	fmt.Printf("MockDestination1 failed to successfully receive event: %+v\n", event)
	return false
}

// MockDestination2 : A destination that introduces random delays (up to 2 seconds)
type MockDestination2 struct{}

func (md *MockDestination2) Send(event Event) bool {
	randDuration := time.Duration(rand.Intn(2000)) * time.Millisecond
	time.Sleep(randDuration)
	fmt.Printf("MockDestination2 successfully received event: %+v\n", event)
	return true
}

// MockDestination3 : A destination that always succeeds but logs the received event
type MockDestination3 struct{}

func (md *MockDestination3) Send(event Event) bool {
	fmt.Printf("MockDestination3 successfully received event: %+v\n", event)
	return true
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
