Differences from the Python flow:

# Install Dependencies

First, you'll need to install the `influxdb-client-go` module.

```
go get github.com/influxdata/influxdb-client-go/v2
```

You will need Go 1.17 or later.

# Create a Token

_No change._

# Initialize Client

Paste following code in your `main.go` file:

```go
package main

import (
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
)

func main() {
	token := os.Getenv("INFLUXDB_TOKEN")
	url := "https://eastus-2-stag.azure.cloud2.influxdata.com"
	client := influxdb2.NewClient(url, token)
}
```

# Write Data

Add the following to your `main` function:

```go
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
```

# Execute a Simple Query

Add the following to your `main` function:

```go
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
```

# Execute an Aggregate Query

Add the following to your `main` function:

```go
org := "gcabbage+stag02-us-east-1@influxdata.com"
queryAPI := client.QueryAPI(org)
query := `from(bucket: "externalwrites")
              |> range(start: -10m)
              |> filter(fn: (r) => r._measurement == "measurement1")
              |> mean()`
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
```