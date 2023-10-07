# Golang_Redis_Event_Delivery_System
A system that receives events from multiple users from an HTTP endpoint and delivers (broadcast) them to various destinations
This application ingests events via an HTTP endpoint, processes them, and attempts to send them to multiple destinations. Events that fail to be delivered are retried with an exponential backoff strategy. The application uses Redis to manage events and integrates robust logging and error handling.


### Prerequisites:
```
Go
Docker
Redis
```
### Getting Started:
#### Clone the Repository:
```bash
git clone https://github.com/jayant99acharya/Golang_Redis_Event_Delivery_System.git
```
```bash
cd Golang_Redis_Event_Delivery_System
```

### Setting up Redis:
#### Pull the Redis Docker Image:
```bash
docker pull redis
```

#### Create a Docker Network:
This network allows containers to communicate with each other.
```bash
docker network create my-network
```
#### Run Redis in a Docker Container:
This command will run a Redis container with the name myredis on the created network, exposing Redis' default port 6379.
```bash
docker run --network=my-network -p 6379:6379 --name myredis -d redis
```
### Setting up and Running the Golang App
#### Build the Docker Image for the Golang App:
Navigate to the directory containing the Dockerfile for the Golang application, then run:
```bash
docker build -t my-golang-app .
```
#### Run the Golang App in a Docker Container:
This will run the Golang application on the shared network, exposing port 8080 for the API.
```bash
docker run --network my-network -p 8080:8080 my-golang-app
```
#### Testing the Golang App:
With both Redis and the Golang app running in their respective Docker containers, you can test the application's functionality:

#### Send an HTTP POST request:
```bash
$ curl -X POST http://localhost:8080/ingest -d '{"userID": "sampleUser", "payload": "samplePayload"}'
```
This will ingest the provided event (userID and payload) into the system, which will then be processed and possibly stored in Redis.


### Testing

The application comes with an integration test suite designed to test its primary functionality and behavior under various conditions.

#### Running the Tests:
```bash
go test ./...
```

## Logging

The application uses the `logrus` package for logging. Monitor logs for potential issues or to observe application behavior.

## Error Notifications

In case of delivery failures beyond the max retry threshold, an email notification is sent to the specified admin email.










