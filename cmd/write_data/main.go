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
	url := "https://eastus-2-stag.azure.cloud2.influxdata.com"
	client := influxdb2.NewClient(url, token)

	// New code
	org := "gcabbage+stag02-us-east-1@influxdata.com"
	bucket := "externalwrites"
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

/* Python source:

bucket="<BUCKET>"

write_api = client.write_api(write_options=SYNCHRONOUS)

for value in range(5):
  point = (
    Point("measurement1")
    .tag("tagname1", "tagvalue1")
    .field("field1", value)
  )
  write_api.write(bucket=bucket, org="gcabbage+stag02-us-east-1@influxdata.com", record=point)
  time.sleep(1) # separate points by 1 second

*/
