// package main implements a sample Go application built on InfluxDB.
//
// This application is designed to illustrate the use of the influxdb-client-go
// module and the facilities of the underlying database; in some cases it omits
// important best practices such as handling errors and authenticating requests.
// Be sure to include those things in any real-world production application!
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	influxdb2http "github.com/influxdata/influxdb-client-go/v2/api/http"
)

// Your app needs the following information:
// - An organization name
// - A host URL
// - A token
// - A bucket name
var (
	// organizationName specifies your InfluxDB organization.
	// Organizations are used by InfluxDB to group resources such as users,
	// tasks, buckets, dashboards and more.
	organizationName = os.Getenv("INFLUXDB_ORGANIZATION")
	// organizationID is used by the task API and is populated
	// by looking up the organizationName at startup.
	organizationID string
	// host is the URL of your InfluxDB instance or Cloud environment.
	// This is also the URL where you reach the UI for your account.
	host = os.Getenv("INFLUXDB_HOST")
	// token appropriately scoped to access the resources needed by your app.
	// For ease of use in this example, you should use an "all access" token.
	// In a production application, you should use a properly scoped token to
	// access only the resources needed by your application and store it securely.
	// More information about permissions and tokens can be found here:
	// https://docs.influxdata.com/influxdb/v2.1/security/tokens/
	token = os.Getenv("INFLUXDB_TOKEN")
	// bucketName specifies an InfluxDB bucket in your organization.
	// A bucket is where you store data, and you can group related data into a bucket.
	// You can also scope permissions to the bucket level as well.
	bucketName = os.Getenv("INFLUXDB_BUCKET")

	// client for accessing InfluxDB
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	queryAPI api.QueryAPI
)

// init sets up the InfluxDB client and its read and write APIs.
func init() {
	client = influxdb2.NewClient(host, token)
	writeAPI = client.WriteAPIBlocking(organizationName, bucketName)
	queryAPI = client.QueryAPI(organizationName)
}

// main starts your Go application and begins listening on port 8080.
func main() {
	// Lookup the organizationID using the organizationName.
	org, err := client.OrganizationsAPI().FindOrganizationByName(context.Background(), organizationName)
	if err != nil {
		log.Fatal(fmt.Errorf("Failed to lookup organization named %q: %v", organizationName, err))
	}
	organizationID = *org.Id

	// Register some routes for your application. Check out the documentation of
	// each function registered below for more details on how it works.
	http.HandleFunc("/", welcome)
	http.HandleFunc("/ingest", ingest) // Ingest application user data.
	http.HandleFunc("/query", query)   // Query application user data.
	http.HandleFunc("/setup", setup)   // Set up a new user of your application.

	// Serve the routes configured above on port 8080.
	// Note that while this app uses Go's HTTP defaults for brevity, a real-world
	// production app exposed on the internet should use a server with properly
	// configured timeouts, certificates, etc.
	http.ListenAndServe(":8080", nil)
}

func welcome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<p>Welcome to your first InfluxDB Application</p>"))
}

// ingest data for a user to InfluxDB.
//
// Note that "user" here refers to a user in your application, not an InfluxDB user.
//
// POST the following data to the /ingest endpoint to test this function:
// {"user_id":"user1", "measurement":"measurement1","field1":1.0}
//
// A point requires at a minimum: A measurement, a field, and a value.
// Where a bucket is similar to a database in a relational database, a measurement is similar
// to a table and a field and its related value are similar to a column and value.
// The user_id will be used to "tag" each point, so that your queries can easily find the
// data for each separate user.
//
// You can write any number of tags and fields in a single point, but only one measurement
// To understand how measurements, tag values, and fields define points and series, follow this link:
// https://awesome.influxdata.com/docs/part-2/influxdb-data-model/
//
// For learning about how to ingest data at scale, and other details, follow this link:
// https://influxdb-client.readthedocs.io/en/stable/usage.html#write
func ingest(w http.ResponseWriter, r *http.Request) {

	// Parse the JSON request body. Production code should authenticate the caller
	// and authorize access to the user identified by the provided user ID.
	var request struct {
		UserID      string  `json:"user_id"`
		Measurement string  `json:"measurement"`
		Field       float64 `json:"field1"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Construct an InfluxDB point from the JSON request suitable for writing.
	point := influxdb2.NewPoint(request.Measurement, map[string]string{
		"user_id": request.UserID,
	}, map[string]interface{}{
		"field1": request.Field,
	}, time.Now())

	// Write the point to InfluxDB using the non-blocking write API.
	if err := writeAPI.WritePoint(r.Context(), point); err != nil {
		handleError(w, err)
		return
	}

	// You can view the data written by this function by navigating to
	// the InfluxDB UI for your account and using the Data Explorer.
}

// query serves down sampled data for a user in JSON format.
//
// Note that "user" here refers to a user in your application, not an InfluxDB user.
//
// POST the following to test this endpoint:
// {"user_id":"user1"}
func query(w http.ResponseWriter, r *http.Request) {

	// Parse the JSON request body. Production code should authenticate the caller
	// and authorize access to the user identified by the provided user ID.
	var request struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Queries can be written in either Flux or InfluxQL.
	// Here we use a parameterized Flux query.
	//
	// Simple queries are in the format of from() |> range() |> filter()
	// Flux can also be used to do complex data transformations as well as integrations.
	// Follow this link to learn more about using Flux:
	// https://awesome.influxdata.com/docs/part-2/introduction-to-flux/
	params := map[string]string{
		"bucket_name": bucketName,
		"user_id":     request.UserID,
	}
	query := `from(bucket: params.bucket_name) 
				|> range(start: -1h) 
				|> filter(fn: (r) => r._measurement == "downsampled")
				|> filter(fn: (r) => r.user_id == params.user_id)
				|> group(columns: ["_field"])
				|> last()`

	// The query API offers the ability to retrieve raw data via QueryRaw and QueryRawWithParams, or
	// a parsed representation via Query and QueryWithParams. We use the latter here.
	tables, err := queryAPI.QueryWithParams(r.Context(), query, params)
	if err != nil {
		handleError(w, err)
		return
	}

	// Use the parsed representation of the query results to iterate
	// over the tables and gather all records into an array.
	type Table struct {
		Records []string `json:"records"`
	}
	var response struct {
		Tables []Table `json:"tables"`
	}
	var currentTable *Table
	for tables.Next() {
		if tables.TableChanged() {
			if currentTable != nil {
				response.Tables = append(response.Tables, *currentTable)
			}
			currentTable = &Table{}
		}
		currentTable.Records = append(currentTable.Records, tables.Record().String())
	}
	if currentTable != nil {
		response.Tables = append(response.Tables, *currentTable)
	}

	// Marshal the response into JSON and return it to the client.
	responseBytes, err := json.Marshal(&response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("ContentType", "application/json")
	w.Write(responseBytes)
}

// setup creates a task owned by the requested user that will down sample their data and write
// the min, max and mean of each field of each measurement to a new measurement every five minutes.
//
// Note that "user" here refers to a user in your application, not an InfluxDB user.
//
// POST the following to test this endpoint:
// {"user_id":"user1"}
func setup(w http.ResponseWriter, r *http.Request) {

	// Parse the JSON request body. Production code should authenticate the caller
	// and authorize access to the user identified by the provided user ID.
	var request struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Format a query that will down sample each field of each measurement included
	// in the data by calculating the mean, min and max and writing the results to
	// a new measurement in the same bucket.
	taskQuery := fmt.Sprintf(`
	data = from(bucket: %q)
		|> range(start: -5m)
		|> filter(fn: (r) => r.user_id == %q)
        |> filter(fn: (r) => r._measurement != "downsampled")
		|> drop(columns: ["_start", "_time", "_stop"])

	max_data = data
		|> max()
		|> map(fn: (r) => ({ r with _field: r._field + "_max"}))

	min_data = data
		|> min()
		|> map(fn: (r) => ({ r with _field: r._field + "_min"}))

	mean_data = data
		|> mean()
		|> map(fn: (r) => ({ r with _field: r._field + "_mean"}))

	union(tables: [max_data, min_data, mean_data])
	 	|> map(fn: (r) => ({ r with _time: now(), _measurement: "downsampled" }))
		|> to(bucket: %q)`,
		bucketName, request.UserID, bucketName)

	name := fmt.Sprintf("%s_task", request.UserID)
	_, err := client.TasksAPI().CreateTaskWithEvery(r.Context(), name, taskQuery, "5m", organizationID)
	if err != nil {
		handleError(w, err)
		return
	}
}

// handleError checks whether the provided error includes a status code and writes
// it to the ResponseWriter if so, otherwise defaulting to an internal server error.
func handleError(w http.ResponseWriter, err error) {
	// You can build on this code to interpret errors from the InfluxDB API and
	// handle them differently, e.g. returning an application error in the event
	// your bucket is not found and the InfluxDB API returns a 404 status.
	if influxErr, ok := err.(*influxdb2http.Error); ok {
		w.WriteHeader(influxErr.StatusCode)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
