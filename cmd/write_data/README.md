# Write Data

A basic snippet showing how to write data to InfluxDB using the [Go client](https://github.com/influxdata/influxdb-client-go).

Requires the following environment variables to be set:
- `INFLUXDB_ORGANIZATION` - The name of your organization
- `INFLUXDB_HOST` - The hostname of the InfluxDB instance or Cloud environment you are using
- `INFLUXDB_TOKEN` - A token with write permissions to the bucket specified in `INFLUXDB_BUCKET`
- `INFLUXDB_BUCKET` - The name of your bucket