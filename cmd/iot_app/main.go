package main

// Port of https://github.com/InfluxCommunity/iot_app
// Using Echo instead of Flask for a webserver.

import (
	"context"
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
)

// HTML templates
type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
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

		// #TODO: validate credentials and handle accordingly.
		return c.String(http.StatusOK, "temp")
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
