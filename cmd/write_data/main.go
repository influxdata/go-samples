package main

import (
	"context"
	"log"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

func main() {
	token := os.Getenv("INFLUXDB_TOKEN")
	url := os.Getenv("INFLUXDB_HOST")
	client := influxdb2.NewClient(url, token)

	org := os.Getenv("INFLUXDB_ORGANIZATION")
	bucket := os.Getenv("INFLUXDB_BUCKET")
	writeAPI := client.WriteAPIBlocking(org, bucket)
	for value := 0; value < 5; value++ {
		tags := map[string]string{
			"tagname1": "tagvalue1",
		}
		fields := map[string]interface{}{
			"field1": value,
		}
		point := write.NewPoint("measurement1", tags, fields, time.Now())

		if err := writeAPI.WritePoint(context.Background(), point); err != nil {
			log.Fatal(err)
		}
	}
}
