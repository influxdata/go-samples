# Basic IOT App

A basic application that demonstrates logging into a local database, as well as making queries and inserts to an InfluxDB database.

The following environment variables are required to be set:

- `INFLUXDB_ORGANIZATION` - The name of your organization
- `INFLUXDB_HOST` - The hostname of the InfluxDB instance or Cloud environment you are using
- `INFLUXDB_BUCKET` - The name of your bucket

This application provides simple query/write utilities and primitive data visualization.

When first starting, you'll want to create a user account via the `Login` -> `Sign Up` page. From here, you can add your InfluxDB tokens for reading and writing. After creating your account, you'll be able to login locally using your set email and password.

After signing in, you'll be able to `Query Data` and `Write Data` using the buttons on screen. Querying data displays a graph containing all of the datapoints in your bucket's first table, and writing will insert a random datapoint into this table.
