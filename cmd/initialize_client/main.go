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

/* Python source:

import os
from influxdb_client import InfluxDBClient, Point, WritePrecision
from influxdb_client.client import SYNCHRONOUS

token = os.environ.get("INFLUXDB_TOKEN")
org = "gcabbage+stag02-us-east-1@influxdata.com"
url = "https://eastus-2-stag.azure.cloud2.influxdata.com"

client = influxdb_client.InfluxDBClient(url=url, token=token, org=org)

*/
