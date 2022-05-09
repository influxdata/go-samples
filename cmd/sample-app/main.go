// package main implements a sample Go application built on InfluxDB.
//
// This application is designed to illustrate the use of the influxdb-client-go
// module and the facilities of the underlying database; in some cases it omits
// important best practices such as handling errors and authenticating requests.
// Be sure to include those things in any real-world production application!
package main

import (
	"encoding/json"
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
	// organizationName is used by InfluxDB to group resources such as tasks, buckets, etc.
	organizationName = os.Getenv("INFLUXDB_ORGANIZATION")
	// host is the URL where your instance of InfluxDB runs.
	// This is also the URL where you reach the UI for your account.
	host = os.Getenv("INFLUXDB_HOST")
	// token appropriately scoped to access the resources needed by your app.
	// For ease of use in this example, we will use an all access token.
	// Note that you should not store the token in source code in a real application, but rather use a proper secrets store.
	// More information about permissions and tokens can be found here:
	// https://docs.influxdata.com/influxdb/v2.1/security/tokens/
	token = os.Getenv("INFLUXDB_TOKEN")
	// bucketName is required for the write_api.
	// A bucket is where you store data, and you can
	// group related data into a bucket.
	// You can also scope permissions to the bucket level as well.
	bucketName = "raw_data_bucket"

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

func main() {
	findOrCreateBucket(bucketName)

	http.HandleFunc("/", welcome)
	http.HandleFunc("/ingest", ingest)
	http.HandleFunc("/query", query)
	http.HandleFunc("/visualize", visualize)
	http.HandleFunc("/tasks", tasks)
	http.HandleFunc("/monitor", monitor)

	// Serve the routes configured above on port 8080.
	// Note that while this app uses Go's HTTP defaults for brevity, a real-world
	// production app should use a server with properly configured timeouts, etc.
	http.ListenAndServe(":8080", nil)
}

func welcome(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("<p>Welcome to your first InfluxDB Application</p>"))
}

// ingest data to InfluxDB.
//
// Post the following data to this function to test:
// {"user_id":"user1", "measurement":"measurement1","field1":1.0}
//
// A point requires at a minimum: A measurement, a field, and a value.
// A measurement is the top level organization for points in a bucket, similar to a table in a relational database.
// A field and its related value are similar to a column and value in a relational database.
// The user_id will be used to "tag" each point, so that your queries can easily find the data for each separate user.
//
// You can write any number of tags and fields in a single point, but only one measurement
// To understand how measurements, tag values, and fields define points and series, follow this link:
// https://awesome.influxdata.com/docs/part-2/influxdb-data-model/
//
// For learning about how to ingest data at scale, and other details, follow this link:
// https://influxdb-client.readthedocs.io/en/stable/usage.html#write
func ingest(w http.ResponseWriter, r *http.Request) {

	// Parse the JSON request body.
	// Your real code should authorize the user, and ensure that the user_id matches the authorization.
	var request struct {
		UserID      string  `json:"user_id"`
		Measurement string  `json:"measurement"`
		Field       float64 `json:"field1"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Transform an InfluxDB point from the JSON request.
	point := influxdb2.NewPoint(request.Measurement, map[string]string{
		"user_id": request.UserID,
	}, map[string]interface{}{
		"field1": request.Field,
	}, time.Now())

	// Write the point to InfluxDB.
	if err := writeAPI.WritePoint(r.Context(), point); err != nil {
		// You can build on this code to interpret errors from the InfluxDB API and
		// handle them differently, e.g. returning an application error in the event
		// your bucket is not found and the InfluxDB API returns a 404 status.
		if influxErr, ok := err.(*influxdb2http.Error); ok {
			w.WriteHeader(influxErr.StatusCode)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	// To view the data that you are writing in the UI, you can use the data explorer
	// Follow this link:
	// TODO: Insert the appropriate /me/ link here.
}

// query serves all data for the user in the last hour in JSON format.
//
// Post the following to test this endpoint:
// {"user_id":"user1"}
func query(w http.ResponseWriter, r *http.Request) {

	// Parse the JSON request body.
	// Your real code should authorize the user, and ensure that the user_id matches the authorization.
	var request struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	//# Queries can be written in either Flux or InfluxQL.
	//# Simple queries are in the format of from() |> range() |> filter()
	//# Flux can also be used to do complex data transformations as well as integrations.
	//# Follow this link to learn more about using Flux:
	//# https://awesome.influxdata.com/docs/part-2/introduction-to-flux/
	//
	//# Set up the arguments for the query parameters
	//params = {"bucket_name": bucket_name, "user_id": user_id}
	params := map[string]string{
		"bucket_name": bucketName,
		"user_id":     request.UserID,
	}
	query := `from(bucket: bucket_name) |> range(start: -1h) |> filter(fn: (r) => r.user_id == user_id)`

	// Execute the query with the query api, and a stream of tables will be returned
	// If it encounters problems, the query() method will throw an ApiException.
	// In this case, we are simply going to return all errors to the user but not handling exceptions
	tables, err := queryAPI.QueryWithParams(r.Context(), query, params)
	if err != nil {
		// You can build on this code to interpret errors from the InfluxDB API and
		// handle them differently, e.g. returning an application error in the event
		// your bucket is not found and the InfluxDB API returns a 404 status.
		if influxErr, ok := err.(*influxdb2http.Error); ok {
			w.WriteHeader(influxErr.StatusCode)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	for tables.Next() {
		tables.
	}

	//# the data will be returned as Python objects so you can iterate the data and do what you want
	//for table in tables:
	//for record in table.records:
	//print(record)
	//
	//# You can use the built in encoder to return results in json
	//output = json.dumps(tables, cls=FluxStructureEncoder, indent=2)
	//return output, 200
	panic("not implemented")
}

// visualize serves a visualization (graph) instead of just data.
//
// Send the username as an argument from the web browser to test this endpoint:
// 127.0.0.1:8080/visualize?user_name=user1
func visualize(w http.ResponseWriter, r *http.Request) {

	//# Your real code should authorize the user, and ensure that the user_id matches the authorization.
	//user_id = request.args.get("user_name")
	//
	//# uncomment the following line and comment out the above line if you prefer to try this without posting the data
	//# user_id = "user1"
	//
	//# Query using Flux as in the /query end point
	//params = {"bucket_name": bucket_name, "user_id": user_id}
	//query = "from(bucket: bucket_name) |> range(start: -1h) |> filter(fn: (r) => r.user_id == user_id)"
	//
	//# This example users plotly and pandas to create the visualization
	//# You can learn more about using InfluxDB with Pandas by following this link:
	//#
	//# InfluxDB supports any visualization library you choose, you can learn more about visualizing data following this link:
	//#
	//data_frame = query_api.query_data_frame(query, organization_name, params=params)
	//graph = px.line(data_frame, x="_time", y="_value", title="my graph")
	//
	//return graph.to_html(), 200
	panic("not implemented")
}

// tasks serves the list of those owned by the requested user.
//
// Post the following to test this endpoint:
// {"user_id":"user1"}
func tasks(w http.ResponseWriter, r *http.Request) {

	//# ensure there is a bucket to copy the data into
	//find_or_create_bucket("processed_data_bucket")
	//
	//# The follow flux will find any values in the specified time range that have a
	//# value of 0.0 and will copy those points into a special bucket.
	//# This demonstrates 2 concepts:
	//# 1. "downsampling", or the ability to easily precompute data so that you can supply low latency
	//#    queries for your UI.
	//#    For more on downsampling, see:
	//#    https://awesome.influxdata.com/docs/part-2/querying-and-data-transformations/#materialized-views-or-downsampling-tasks
	//# 2. "alerting", or the ability to send a notification based on certain values and conditions.
	//#    For example, rather than writing the data to a new bucket, you can use http.post() to call back your application
	//#    or a different service.
	//#    To see the full power of the alerting system, see:
	//#    https://awesome.influxdata.com/docs/part-3/checks-and-notifications/
	//query = """
	//option task = {{name: "{}_task", every: 1m}}
	//from(bucket: "{}")
	//|> range(start: -1m)
	//|> filter(fn: (r) => r.user_id == "{}")
	//|> filter(fn: (r) => r._value == 0.0)
	//|> to(bucket: "processed_data_bucket")
	//"""
	//
	//if request.method == "POST":
	//# Your real code should authorize the user, and ensure that the user_id matches the authorization.
	//user_id = request.json["user_id"]
	//# If you prefer to try this without posting the data,
	//# uncomment the following line and comment out the above line
	//# user_id = "user1"
	//
	//# Update the query specific to the user id
	//q = query.format(user_id, bucket_name, user_id)
	//
	//# Prepare the REST API call.
	//# In some cases, the REST API is simpler to use than the client API
	//# Refer to the REST API docs to see how to manage tasks:
	//# https://docs.influxdata.com/influxdb/cloud/api/#operation/PostTasks
	//data = {"flux": q, "org": organization_name}
	//url = urljoin(host, "/api/v2/tasks")
	//
	//headers = {
	//"Authorization": f"Token {token}",
	//"Content-Type": "application/json",
	//}
	//response = requests.post(url, headers=headers, data=json.dumps(data))
	//if response.status_code == 201:
	//r = json.loads(response.text)
	//
	//# This will return the task id, which your application should store so that it can refer to it later
	//# for managing tasks
	//return {"task_id": r["id"]}, 201
	//else:
	//return response.text, response.status_code
	panic("not implemented")
}

// monitor serves information related to how your application is behaving in InfluxDB.
//
// InfluxDB includes functionality designed to help you programmatically manage your instances.
// This page provides basic insights into usage, and tasks. Much more related functionality exists.
// There is a template that you can install into your account, to learn more, follow this link:
// https://www.influxdata.com/blog/tldr-influxdb-tech-tips-using-and-understanding-the-influxdb-cloud-usage-template/
//
// Your code should verify that the person viewing this has proper authorization.
func monitor(w http.ResponseWriter, r *http.Request) {

	//# The following flux query will retrieve the 3 kinds of usage data available
	//# and combine the data into a single table for ease of formatting.
	//# For more information about the usage.from() function, see the following:
	//# https://docs.influxdata.com/flux/v0.x/stdlib/experimental/usage/from/
	//
	//query = """
	//import "experimental/usage"
	//usage.from(start: -1h, stop: now())
	//|> toFloat()
	//|> group(columns: ["_measurement"])
	//|> sum()
	//"""
	//tables = query_api.query(query, org=organization_name)
	//html = "<H1>usage</H1><TABLE><TR><TH>usage type</TH><TH>value</TH></TR>"
	//for table in tables:
	//for record in table:
	//mes = record["_measurement"]
	//val = record["_value"]
	//html += f"<TR><TD>{mes}</TD><TD>{val}</TD></TR>"
	//html += "</TABLE>"
	//
	//# This part of the function looks at task health.
	//# Tasks allow you to run code periodically within InfluxDB.
	//# For an overview of tasks, see
	//# https://docs.influxdata.com/influxdb/cloud/process-data/get-started/
	//
	//# It is very useful to know if your tasks are running and succeeding or not, and to alert on those conditions.
	//# This section uses the tasks_api that comes with the client library
	//# Documentation on this library is available here:
	//# https://influxdb-client.readthedocs.io/en/stable/api.html#tasksapi
	//tasks_api = client.tasks_api()
	//
	//# list all the tasks
	//tasks = tasks_api.find_tasks()
	//
	//html += "<H1>tasks</H1><TABLE><TR><TH>name</TH><TH>status</TH><TH>last run</TH><TH>last run status</TH></TR>"
	//
	//# Each task has a run log, accessed through the get_runs() function
	//# This code checks if each task is enabled, and if so, checks the status of its last run
	//# For active tasks, format the status report
	//for task in tasks:
	//started_at = ""
	//run_status = ""
	//if task.status == "active":
	//runs = tasks_api.get_runs(task.id, limit=1)
	//if len(runs) > 0:  # new tasks my not have any runs yet
	//run = runs[0]
	//started_at = run.started_at
	//run_status = run.status
	//html += f"<TR><TD>{task.name}</TD><TD>{task.status}</TD><TD>{started_at}</TD><TD>{run_status}</TD></TR></BR>"
	//
	//if len(tasks) == 0:
	//html += "<TR><TD>no tasks</TD><TR>"
	//html += "</TABLE>"
	//
	//return html, 200
	panic("not implemented")
}

// findOrCreateBucket is it does not exist.
func findOrCreateBucket(name string) {

}
