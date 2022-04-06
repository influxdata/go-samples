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
	url := "https://eastus-2-stag.azure.cloud2.influxdata.com"
	client := influxdb2.NewClient(url, token)

	// New code
	org := "gcabbage+stag02-us-east-1@influxdata.com"
	queryAPI := client.QueryAPI(org)
	query := `from(bucket: "externalwrites")
				|> range(start: -10m)
 				|> filter(fn: (r) => r._measurement == "measurement1")`
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

/* Python source:

query_api = client.query_api()

query = """from(bucket: "externalwrites")
 |> range(start: -10m)
 |> filter(fn: (r) => r._measurement == "measurement1")"""
tables = query_api.query(query, org="gcabbage+stag02-us-east-1@influxdata.com")

for table in tables:
  for record in table.records:
    print(record)

*/
