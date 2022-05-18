package main

import (
	"context"
	"fmt"
	"log"
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func main() {
	token := os.Getenv("INFLUXDB_TOKEN")
	url := os.Getenv("INFLUXDB_HOST")
	client := influxdb2.NewClient(url, token)

	org := os.Getenv("INFLUXDB_ORGANIZATION")
	queryAPI := client.QueryAPI(org)
	bucket := os.Getenv("INFLUXDB_BUCKET")
	query := fmt.Sprintf(`from(bucket: %q)
				|> range(start: -10m)
 				|> filter(fn: (r) => r._measurement == "measurement1")`, bucket)
	results, err := queryAPI.Query(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}
	for results.Next() {
		fmt.Println(results.Record())
	}
	if err := results.Err(); err != nil {
		log.Fatal(err)
	}
}
