package main

import (
	"context"
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func main() {
	token := os.Getenv("INFLUXDB_TOKEN")
	url := os.Getenv("INFLUXDB_HOST")
	client := influxdb2.NewClient(url, token)
	client.Ping(context.Background())
}
