// Package main implements a basic IoT app that features a local login system
// for managing tokens, as well as simple querying and datapoint inserting.
// Note that the login system is extraordinally simple, allowing for just a single
// login at a time. This is for demonstration purposes.
// It is a port of the Python sample found here: https://github.com/InfluxCommunity/iot_app
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	_ "github.com/mattn/go-sqlite3" // Need the sqlite3 driver.
)

type User struct {
	valid      bool
	name       string // Local name from login.
	email      string // Local email from login.
	readToken  string
	writeToken string
}

var (
	activeUser  User
	readClient  influxdb2.Client
	writeClient influxdb2.Client
	queryJson   string

	hostUrl = os.Getenv("INFLUXDB_HOST")
	orgId   = os.Getenv("INFLUXDB_ORGANIZATION_ID")
	bucket  = os.Getenv("INFLUX_BUCKET")
)

const loginDatabase = "logins.db"

// getLoginDB connects to the local login database. Creates one with a default account if there isn't one.
func getLoginDB() (*sql.DB, error) {
	// If we don't have a login database yet, create one with a default user account.
	if _, err := os.Stat(loginDatabase); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Failed to find logins database, creating a new one.")
		db, err := sql.Open("sqlite3", loginDatabase)
		if err != nil {
			return nil, err
		}

		create := `CREATE TABLE user(
			id INTEGER NOT NULL,
			email VARCHAR(100),
			password VARCHAR(100),
			name VARCHAR(1000),
			readToken VARCHAR(100),
			writeToken VARCHAR(100),
			PRIMARY KEY (id),
			UNIQUE (email))`
		_, err = db.Exec(create)
		if err != nil {
			return db, fmt.Errorf("login table create failed: %s", err)
		}

		const defaultLoginMessage = `
Creating default user with login:
	Email: mickey@example.com
	Password: pass
Note that this account will not be able to access your influxdb organization.`

		fmt.Println(defaultLoginMessage)

		insert := `INSERT INTO user VALUES(
			1,
			'mickey@example.com',
			'sha256$d74ff0ee8da3b9806b18c877dbf29bbde50b5bd8e4dad7a3a725000feb82e8f1',
			'mickey',
			'my_read_token',
			'my_write_token')`
		_, err = db.Exec(insert)
		if err != nil {
			return db, fmt.Errorf("login user insert failed: %q", err)
		}
	}

	// Database already exists, just open it.
	return sql.Open("sqlite3", loginDatabase)
}

func tryLoginCredentials(db *sql.DB, user string, plainPassword string) error {
	result, err := db.Query(`SELECT * FROM user WHERE email=$1`, user)
	if err != nil {
		return fmt.Errorf("failed to send query: %q", err)
	}
	defer result.Close()

	for result.Next() {
		var (
			id                                           int
			email, password, name, readToken, writeToken string
		)
		err = result.Scan(&id, &email, &password, &name, &readToken, &writeToken)
		if err != nil {
			return fmt.Errorf("result scan failed: %q", err)
		}

		hasher := sha256.New()
		hasher.Write([]byte(plainPassword))
		hash := hasher.Sum(nil)

		password = strings.TrimPrefix(password, "sha256$")
		decoded, err := hex.DecodeString(password)
		if err != nil {
			return fmt.Errorf("failed to decode password hash: %q", err)
		}

		if !bytes.Equal(hash, decoded) {
			return errors.New("incorrect password")
		}

		var newUser User
		newUser.valid = true
		newUser.name = name
		newUser.email = email
		newUser.readToken = readToken
		newUser.writeToken = writeToken

		activeUser = newUser

		// Update our read/write clients since we just retrieved the tokens.
		readClient = influxdb2.NewClient(hostUrl, readToken)
		writeClient = influxdb2.NewClient(hostUrl, writeToken)

		return nil
	}

	return errors.New("failed to find any matching user account emails")
}

func registerUser(db *sql.DB, email string, name string, password string, readToken string, writeToken string) error {
	hasher := sha256.New()
	hasher.Write([]byte(password))
	hash := hasher.Sum(nil)
	passwordHash := hex.EncodeToString(hash)
	passwordHash = strings.Join([]string{"sha256$", passwordHash}, "")

	insert := `INSERT INTO user VALUES(
		$1,
		$2,
		$3,
		$4,
		$5,
		$6)`
	_, err := db.Exec(insert, rand.Int(), email, passwordHash, name, readToken, writeToken)

	return err
}

// queryData runs a simple query that fetches all data in the past 100 hours, returns a query table result.
func queryData(cl influxdb2.Client) (*api.QueryTableResult, error) {
	queryApi := cl.QueryAPI(orgId)

	params := map[string]string{
		"bucket_name": bucket,
	}
	query := `from(bucket: params.bucket_name)
				|> range(start: -100h)`
	results, err := queryApi.QueryWithParams(context.Background(), query, params)
	if err != nil {
		return nil, fmt.Errorf("failed to run db query: %q", err)
	}

	return results, nil
}

// writeData writes a random data point.
func writeData(cl influxdb2.Client) error {
	writeApi := cl.WriteAPIBlocking(orgId, bucket)

	tags := map[string]string{
		"tagname1": "tagvalue1",
	}
	const numberRange = 128
	fields := map[string]interface{}{
		"field1": rand.Float32()*numberRange - numberRange*0.5,
	}

	point := write.NewPoint("measurement1", tags, fields, time.Now())
	if err := writeApi.WritePoint(context.Background(), point); err != nil {
		return fmt.Errorf("failed to run db write: %q", err)
	}

	return nil
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	t, _ := template.ParseFiles(fmt.Sprintf("templates/%s.html", tmpl))
	t.Execute(w, data)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "index", nil)
}

func loginHandler(db *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			renderTemplate(w, "login", nil)
		case "POST":
			email := r.FormValue("email")
			password := r.FormValue("password")
			fmt.Printf("Login post, retrieved credientials: email:%s\n", email)

			// Query the login database to see if the credentials match.
			if err := tryLoginCredentials(db, email, password); err == nil {
				fmt.Println("Login success")
				http.Redirect(w, r, "profile", http.StatusSeeOther)
			} else {
				fmt.Printf("Login failed: %q\n", err)
				http.Error(w, "Invalid login", http.StatusForbidden)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func profileHandler(w http.ResponseWriter, r *http.Request) {
	if !activeUser.valid {
		fmt.Println("Not logged in, redirecting to login page.")
		http.Redirect(w, r, "login", http.StatusSeeOther)
	}

	renderTemplate(w, "profile", map[string]interface{}{
		"name":      activeUser.name,
		"queryJson": queryJson,
	})
}

func queryDataHandler(w http.ResponseWriter, r *http.Request) {
	data, err := queryData(readClient)
	if err != nil {
		fmt.Printf("Query failed: %q\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("%q", err)))
		return
	}

	// Encoding expected by Plotly.js.
	type GraphData struct {
		X []int     `json:"x"`
		Y []float64 `json:"y"`
	}

	graphData := make([]GraphData, 1)

	// Basic single-table graphing of x,y points.

	firstTable := true
	counter := 0
	for data.Next() {
		if data.TableChanged() && !firstTable {
			break // We only care about the first table here. Could draw a graph per table too.
		}
		firstTable = false
		pairs := strings.Split(data.Record().String(), ",")
		valueString := ""
		for _, pair := range pairs {
			kv := strings.Split(pair, ":")
			if kv[0] != "_value" {
				continue
			}
			valueString = kv[1]
		}
		if valueString == "" {
			break // invalid data
		}

		// We're only interested in the _value entries here.
		value, _ := strconv.ParseFloat(valueString, 32)
		graphData[0].X = append(graphData[0].X, counter)
		graphData[0].Y = append(graphData[0].Y, value)

		counter += 1
	}

	jsonBytes, err := json.Marshal(graphData)
	if err != nil {
		fmt.Printf("Query failed when marshalling data: %q\n", err)
		return
	}

	// Write out the json to the http body, this will update the HTML.
	json.NewEncoder(w).Encode(string(jsonBytes))
}

func writeDataHandler(w http.ResponseWriter, r *http.Request) {
	err := writeData(writeClient)
	if err != nil {
		fmt.Printf("Write failed: %q\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("%q", err)))
		return
	}
}

func signupHandler(db *sql.DB) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			renderTemplate(w, "signup", nil)
		case "POST":
			email := r.FormValue("email")
			name := r.FormValue("name")
			password := r.FormValue("password")
			readToken := r.FormValue("readToken")
			writeToken := r.FormValue("writeToken")

			fmt.Printf("Registering new user: email=%s, name=%s, readToken=%s, writeToken=%s\n",
				email, name, readToken, writeToken)

			if err := registerUser(db, email, name, password, readToken, writeToken); err != nil {
				fmt.Println("Failed to register user:", err)
				http.Error(w, "Failed to register user.", http.StatusBadRequest)
			} else {
				http.Redirect(w, r, "login", http.StatusSeeOther)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func setupWebHandlers(db *sql.DB) {
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/login", loginHandler(db))
	http.HandleFunc("/profile", profileHandler)
	http.HandleFunc("/graph_query_data", queryDataHandler)
	http.HandleFunc("/graph_write_data", writeDataHandler)
	http.HandleFunc("/signup", signupHandler(db))
}

func main() {
	activeUser.valid = false
	db, err := getLoginDB()
	if err != nil {
		log.Fatalf("Get login db failed: %q", err)
	}
	defer db.Close()

	// Make sure the host URL has a scheme, and default to https if not.
	parsedUrl, err := url.Parse(hostUrl)
	if err != nil {
		log.Fatalf("Host URL parsing failed: %q", err)
	}
	if len(parsedUrl.Path) == 0 {
		log.Fatalf("Host URL does not contain a valid path.")
	}
	if !strings.EqualFold(parsedUrl.Scheme, "http") && !strings.EqualFold(parsedUrl.Scheme, "https") {
		parsedUrl.Scheme = "https"
	}
	hostUrl = parsedUrl.String()

	fmt.Println("Starting server at http://localhost:8080")

	setupWebHandlers(db)
	http.ListenAndServe(":8080", nil)
}
