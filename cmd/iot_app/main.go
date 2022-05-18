package main

// Port of https://github.com/InfluxCommunity/iot_app
// Using Echo instead of Flask for a webserver.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/mattn/go-sqlite3" // Need the sqlite3 driver.
)

type User struct {
	valid      bool
	name       string // Local name from login.
	email      string // Local email from login.
	readToken  string
	writeToken string
}

// Maybe not the best choice...
var activeUser User

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

func tryLoginCredentials(user string, password string) bool {
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

		var newUser User
		newUser.valid = true
		newUser.name = name
		newUser.email = email
		newUser.readToken = readToken
		newUser.writeToken = writeToken

		activeUser = newUser

		return true
	}

	// No matches.
	return false
}

// HTML templates
type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	// https://echo.labstack.com/guide/templates/#advanced---calling-echo-from-templates
	if viewContext, isMap := data.(map[string]interface{}); isMap {
		viewContext["reverse"] = c.Echo().Reverse
	}

	return t.templates.ExecuteTemplate(w, name, data)
}

func setupWebHandlers(e *echo.Echo) {
	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "index.html", nil)
	})
	e.GET("/login", func(c echo.Context) error {
		return c.Render(http.StatusOK, "login.html", nil)
	})
	e.POST("/login", func(c echo.Context) error {
		// Attempt to verify credientials and redirect to profile.html if successful.
		email := c.FormValue("email")
		password := c.FormValue("password")
		fmt.Printf("Login post, retrieved credientials: email:%s, password:%s\n", email, password)

		// Query the login database to see if the credentials match.
		if tryLoginCredentials(email, password) {
			fmt.Println("SUCCESS")
			return c.Redirect(http.StatusSeeOther, "profile")
		} else {
			fmt.Println("FAILED")
			return c.String(http.StatusForbidden, "Invalid login")
		}
	})
	e.GET("/profile", func(c echo.Context) error {
		// Maybe a better way to do this. Flask has @login_required.
		if !activeUser.valid {
			fmt.Println("Not logged in, redirecting to login page.")
			return c.Redirect(http.StatusSeeOther, "login")
		}

		return c.Render(http.StatusOK, "profile.html", map[string]interface{}{
			"name": activeUser.name,
		})
	})

	e.GET("/graph_query_data", func(c echo.Context) error {
		// ???
		return c.String(http.StatusOK, "query data..?")
	})
}

func createWebserver() {
	const port = 5000 // Using same port as Flask.

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	t := &Template{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
	e.Renderer = t
	setupWebHandlers(e)
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%d", port)))
}

func getEnvVar(name string, fallback string) string {
	v := os.Getenv(name)
	if len(v) == 0 {
		if len(fallback) == 0 {
			fmt.Println("Missing required environment variable", name)
			panic(nil)
		}
		fmt.Println("Missing environment variable", name, ", defaulting to", fallback)
		v = fallback
	} else {
		fmt.Println("Found environment variable", name, "=", v)
	}

	return v
}

func main() {
	activeUser.valid = false
	go createDefaultLoginDB()

	// Run the webserver in a goroutine.
	go createWebserver()

	// Retrieve variables from the environment or use defaults.
	url := getEnvVar("INFLUXDB_URL", "twodotoh-dev-andrew20220517115401.remocal.influxdev.co")
	orgId := getEnvVar("INFLUXDB_ORGID", "9c5955fc99a60b8f")
	bucket := getEnvVar("INFLUX_BUCKET", "devbucket")
	token := getEnvVar("INFLUXDB_TOKEN", "")

	// Url formatting.
	httpsPrefix := "https://"
	if !strings.Contains(url, httpsPrefix) {
		url = strings.Join([]string{httpsPrefix, url}, "")
	}

	client := influxdb2.NewClient(url, token)
	queryApi := client.QueryAPI(orgId)
	// Could use the authorizations API to get auth details from the actual database.
	//authApi := client.AuthorizationsAPI()

	query := fmt.Sprintf(`from(bucket: "%s")
							|> range(start: -100h)`, bucket)

	results, err := queryApi.Query(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}

	// #TODO: feed into a channel, which is read in the profile GET?
	for results.Next() {
		fmt.Println("RESULT: ", results.Record())
	}

	if err := results.Err(); err != nil {
		log.Fatal(err)
	}

	// Sleep forever, don't want to kill the webserver as soon as we launch it.
	select {}
}
