package main

// Port of https://github.com/InfluxCommunity/iot_app

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
	"os"
	"strings"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
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

var activeUser User
var readClient influxdb2.Client
var writeClient influxdb2.Client
var url string = os.Getenv("INFLUXDB_HOST")
var orgId string = os.Getenv("INFLUXDB_ORGANIZATION_ID")
var bucket string = os.Getenv("INFLUX_BUCKET")
var queryJson string

const loginDatabase = "logins.db"

func createDefaultLoginDB() {
	// If we don't have a login database yet, create one with a default user account.
	if _, err := os.Stat(loginDatabase); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Failed to find logins database, creating a new one.")
		db, err := sql.Open("sqlite3", loginDatabase)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()

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
			fmt.Printf("Login table create failed: %q\n", err)
			return
		}

		fmt.Println()
		fmt.Println("Creating default user with login:\n\tEmail: mickey@mouse.com\n\tPassword: pass")
		fmt.Println("Note that this account will not be able to access your influxdb organization.")
		fmt.Println()

		insert := `INSERT INTO user VALUES(
			1,
			'mickey@mouse.com',
			'sha256$d74ff0ee8da3b9806b18c877dbf29bbde50b5bd8e4dad7a3a725000feb82e8f1',
			'mickey',
			'fVYoLl13o0ET6rhOEfpZoKcoOWofA9GE-Dv5P_EWI41fguTOoLuVD5HeGVEfgRVeF9xnYxh-sLcZEXGBkEFuWQ==',
			'xOsoCvepHZzTLdr4WtWyoKNZ8KB-fzJ_4fvQpHIWS8GpBH7GqPP6dyock2cz1oVt5zar--N0AQ6frYBOYfevZg==')`
		_, err = db.Exec(insert)
		if err != nil {
			fmt.Printf("Login user insert failed: %q\n", err)
			return
		}
	}
}

func tryLoginCredentials(user string, plainPassword string) bool {
	db, err := sql.Open("sqlite3", loginDatabase)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	query := fmt.Sprintf(`SELECT * FROM user WHERE email='%s'`, user)
	result, err := db.Query(query)
	if err != nil {
		log.Fatal(err)
	}
	defer result.Close()

	for result.Next() {
		var id int
		var email string
		var password string
		var name string
		var readToken string
		var writeToken string
		err = result.Scan(&id, &email, &password, &name, &readToken, &writeToken)
		if err != nil {
			log.Fatal(err)
		}

		hasher := sha256.New()
		hasher.Write([]byte(plainPassword))
		hash := hasher.Sum(nil)

		password = strings.TrimPrefix(password, "sha256$")
		decoded, err := hex.DecodeString(password)
		if err != nil {
			fmt.Println("Failed to decode password hash:", err)
			return false
		}

		if bytes.Compare(hash, decoded) != 0 {
			fmt.Println("Incorrect password.")
			return false
		}

		var newUser User
		newUser.valid = true
		newUser.name = name
		newUser.email = email
		newUser.readToken = readToken
		newUser.writeToken = writeToken

		activeUser = newUser

		// Update our read/write clients since we just retrieved the tokens.
		readClient = influxdb2.NewClient(url, readToken)
		writeClient = influxdb2.NewClient(url, writeToken)

		return true
	}

	// No matches.
	return false
}

func registerUser(email string, name string, password string, readToken string, writeToken string) error {
	db, err := sql.Open("sqlite3", loginDatabase)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	hasher := sha256.New()
	hasher.Write([]byte(password))
	hash := hasher.Sum(nil)
	passwordHash := hex.EncodeToString(hash)

	insert := fmt.Sprintf(`INSERT INTO user VALUES(
		%d,
		'%s',
		'sha256$%s',
		'%s',
		'%s',
		'%s')`,
		rand.Int(), email, passwordHash, name, readToken, writeToken)
	_, err = db.Exec(insert)

	return err
}

// Returns a json representation of the query.
func queryData(cl influxdb2.Client) string {
	queryApi := cl.QueryAPI(orgId)

	query := fmt.Sprintf(`from(bucket: "%s")
							|> range(start: -100h)`, bucket)
	results, err := queryApi.Query(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}

	type Table struct {
		Metadata string   `json:"metadata"`
		Records  []string `json:"records"`
	}
	var response struct {
		Tables []Table `json:"tables"`
	}
	var currentTable *Table
	for results.Next() {
		if results.TableChanged() || currentTable == nil {
			currentTable = &Table{
				Metadata: results.TableMetadata().String(),
			}
			response.Tables = append(response.Tables, *currentTable)
		}
		currentTable.Records = append(currentTable.Records, results.Record().String())
	}

	responseBytes, err := json.Marshal(&response)
	if err != nil {
		return "Error"
	}

	return string(responseBytes)
}

// Writes a random data point.
func writeData(cl influxdb2.Client) {
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
		log.Fatal(err)
	}
}

func renderTemplate(w http.ResponseWriter, tmpl string, data interface{}) {
	t, _ := template.ParseFiles(fmt.Sprintf("templates/%s.html", tmpl))
	t.Execute(w, data)
}

func setupWebHandlers() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		renderTemplate(w, "index", nil)
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			renderTemplate(w, "login", nil)
		case "POST":
			email := r.FormValue("email")
			password := r.FormValue("password")
			fmt.Printf("Login post, retrieved credientials: email:%s, password:%s\n", email, password)

			// Query the login database to see if the credentials match.
			if tryLoginCredentials(email, password) {
				fmt.Println("Login success")
				http.Redirect(w, r, "profile", http.StatusSeeOther)
			} else {
				fmt.Println("Login failed")
				http.Error(w, "Invalid login", http.StatusForbidden)
			}
		}
	})

	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		if !activeUser.valid {
			fmt.Println("Not logged in, redirecting to login page.")
			http.Redirect(w, r, "login", http.StatusSeeOther)
		}

		w.WriteHeader(http.StatusOK)
		renderTemplate(w, "profile", map[string]interface{}{
			"name":      activeUser.name,
			"queryJson": queryJson,
		})
	})

	http.HandleFunc("/graph_query_data", func(w http.ResponseWriter, r *http.Request) {
		data := queryData(readClient)
		queryJson = data
		encoder := json.NewEncoder(w)
		w.WriteHeader(http.StatusOK)
		encoder.Encode(queryJson)
	})

	http.HandleFunc("/graph_write_data", func(w http.ResponseWriter, r *http.Request) {
		writeData(writeClient)
		w.WriteHeader(http.StatusOK)
	})

	http.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.WriteHeader(http.StatusOK)
			renderTemplate(w, "signup", nil)
		case "POST":
			email := r.FormValue("email")
			name := r.FormValue("name")
			password := r.FormValue("password")
			readToken := r.FormValue("readToken")
			writeToken := r.FormValue("writeToken")

			fmt.Printf("Registering new user: email=%s, name=%s, password=%s, readToken=%s, writeToken=%s\n",
				email, name, password, readToken, writeToken)

			if err := registerUser(email, name, password, readToken, writeToken); err != nil {
				fmt.Println("Failed to register user:", err)
				http.Error(w, "Failed to register user.", http.StatusBadRequest)
			} else {
				http.Redirect(w, r, "login", http.StatusSeeOther)
			}
		}
	})
}

func main() {
	activeUser.valid = false
	go createDefaultLoginDB()

	// Url formatting.
	httpsPrefix := "https://"
	if !strings.Contains(url, httpsPrefix) {
		url = strings.Join([]string{httpsPrefix, url}, "")
	}

	fmt.Println("Starting server at http://localhost:8080")

	setupWebHandlers()
	http.ListenAndServe(":8080", nil)
}
