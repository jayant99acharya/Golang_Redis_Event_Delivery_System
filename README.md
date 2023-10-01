# Golang_Redis_Event_Delivery_System
A system that receives events from multiple users from an HTTP endpoint and delivers (broadcast) them to various destinations


docker pull redis
docker run --name redis-dev-1 -p 6379:6379 -d redis
go run main.go

