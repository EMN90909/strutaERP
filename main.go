package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	app := NewApp("platform_data.json")
	app.Load()

	mux := http.NewServeMux()
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	mux.HandleFunc("/", app.HandleIndex)
	mux.HandleFunc("/setup", app.HandleSetup)
	mux.HandleFunc("/login", app.HandleLogin)
	mux.HandleFunc("/logout", app.HandleLogout)
	mux.HandleFunc("/dashboard", app.RequireLogin(app.HandleDashboard))
	mux.HandleFunc("/users", app.RequireLogin(app.HandleUsers))
	mux.HandleFunc("/activity", app.RequireLogin(app.HandleActivity))
	mux.HandleFunc("/tables", app.RequireLogin(app.HandleTables))
	mux.HandleFunc("/features", app.RequireLogin(app.HandleFeatures))
	mux.HandleFunc("/api-keys", app.RequireLogin(app.HandleAPIKeys))
	mux.HandleFunc("/api-keys/create", app.RequireLogin(app.HandleCreateAPIKey))

	mux.HandleFunc("/api/v1/status", app.HandleAPIStatus)
	mux.HandleFunc("/api/v1/tables", app.HandleAPITables)
	mux.HandleFunc("/api/v1/tables/", app.HandleAPITableRecords)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Println("Server running at http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
