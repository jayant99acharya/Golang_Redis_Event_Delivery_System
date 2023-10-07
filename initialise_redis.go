package main

import (
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
)

// initializeRedis sets up a connection to the Redis server.
func initializeRedis() {
	myredis := "myredis:6379" // Redis server address
	client := redis.NewClient(&redis.Options{
		Addr:     myredis,
		Password: "", // No password
		DB:       0,  // Default DB
	})
	fmt.Println("Initialised redis", myredis)
	rdb = &RedisClientWrapper{Client: client}

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Error initializing Redis: %v", err)
	}
}
