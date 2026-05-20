package main

import (
	"net/http"
	"strings"
)

func (app *App) HandleIndex(w http.ResponseWriter, r *http.Request) {
	app.mu.Lock()
	hasSuper := app.hasSuperuserLocked()
	app.mu.Unlock()

	if !hasSuper {
		http.Redirect(w, r, "/setup", http.StatusSeeOther)
		return
	}
	if app.CurrentUser(r) != nil {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *App) HandleSetup(w http.ResponseWriter, r *http.Request) {
	app.mu.Lock()
	hasSuper := app.hasSuperuserLocked()
	app.mu.Unlock()

	if hasSuper {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if r.Method == http.MethodGet {
		app.Render(w, "setup", nil)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	if name == "" || email == "" || password == "" {
		app.Render(w, "setup", map[string]interface{}{"Error": "All fields are required."})
		return
	}

	app.mu.Lock()
	user := User{
		ID:           newID("usr"),
		Name:         name,
		Email:        email,
		PasswordHash: hashPassword(password),
		Role:         "superuser",
		CreatedAt:    now(),
	}
	app.Data.Users = append(app.Data.Users, user)
	app.logActivityLocked(user.ID, "", r, "SUPERUSER_CREATED", "Initial platform owner account created")
	app.saveLocked()
	app.createSession(w, user.ID)
	app.mu.Unlock()

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (app *App) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		app.Render(w, "login", nil)
		return
	}

	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")

	app.mu.Lock()
	defer app.mu.Unlock()
	for _, u := range app.Data.Users {
		if u.Email == email && u.PasswordHash == hashPassword(password) {
			app.logActivityLocked(u.ID, "", r, "LOGIN", "User signed in")
			app.saveLocked()
			app.createSession(w, u.ID)
			http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
			return
		}
	}
	app.Render(w, "login", map[string]interface{}{"Error": "Invalid email or password."})
}

func (app *App) HandleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session"); err == nil {
		app.mu.Lock()
		delete(app.Sessions, cookie.Value)
		app.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: "session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *App) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Render(w, "dashboard", map[string]interface{}{
		"User":       user,
		"Users":      app.Data.Users,
		"APIKeys":    app.Data.APIKeys,
		"Activities": reverseActivities(app.Data.Activities),
		"Tables":     tableSummaries(app.Data.Tables),
		"Active":     "dashboard",
	})
}

func (app *App) HandleUsers(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Render(w, "users", map[string]interface{}{"User": user, "Users": app.Data.Users, "Active": "users"})
}

func (app *App) HandleActivity(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Render(w, "activity", map[string]interface{}{"User": user, "Activities": reverseActivities(app.Data.Activities), "Active": "activity"})
}

func (app *App) HandleTables(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Render(w, "tables", map[string]interface{}{"User": user, "Tables": tableSummaries(app.Data.Tables), "Active": "tables"})
}

func (app *App) HandleFeatures(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.Render(w, "features", map[string]interface{}{"User": user, "FeatureGroups": platformFeatures(), "Active": "features"})
}

func (app *App) HandleAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Render(w, "api_keys", map[string]interface{}{"User": user, "APIKeys": app.Data.APIKeys, "Active": "api_keys"})
}

func (app *App) HandleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	if user == nil || user.Role != "superuser" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	keyType := strings.TrimSpace(r.FormValue("type"))
	description := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		name = "Untitled API Key"
	}

	privileges := []string{}
	switch keyType {
	case "sdk":
		privileges = []string{"auth", "storage", "realtime"}
	case "anon":
		privileges = []string{"external_tables_create", "external_tables_read", "external_tables_update", "external_tables_delete"}
	default:
		http.Error(w, "Invalid key type", http.StatusBadRequest)
		return
	}

	apiKey := APIKey{
		ID:          newID("key"),
		Name:        name,
		Key:         "fh_" + keyType + "_" + randomToken(32),
		Type:        keyType,
		Privileges:  privileges,
		CreatedAt:   now(),
		CreatedBy:   user.ID,
		Description: description,
	}

	app.mu.Lock()
	app.Data.APIKeys = append(app.Data.APIKeys, apiKey)
	app.logActivityLocked(user.ID, apiKey.ID, r, "API_KEY_CREATED", "Created "+keyType+" API key: "+name)
	app.saveLocked()
	app.mu.Unlock()

	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}
