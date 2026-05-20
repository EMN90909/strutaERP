package main

import (
	"encoding/json"
	"fmt"
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
	app.ensureDefaultProjectLocked()
	app.Data.Projects[0].CreatedBy = user.ID
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
		"Projects":   projectSummaries(app.Data.Projects),
		"Active":     "dashboard",
		"BaseURL":    requestBaseURL(r),
	})
}

func (app *App) HandleCreateProject(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	if user == nil || user.Role != "superuser" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	if name == "" {
		name = "Untitled Project"
	}
	slug := cleanSlug(name)

	app.mu.Lock()
	baseSlug := slug
	for n := 2; ; n++ {
		exists := false
		for _, p := range app.Data.Projects {
			if p.Slug == slug {
				exists = true
				break
			}
		}
		if !exists {
			break
		}
		slug = fmt.Sprintf("%s_%d", baseSlug, n)
	}
	project := Project{
		ID:          newID("prj"),
		Name:        name,
		Slug:        slug,
		Description: description,
		Tables:      map[string][]interface{}{},
		Schemas:     map[string][]string{},
		CreatedAt:   now(),
		CreatedBy:   user.ID,
	}
	app.Data.Projects = append(app.Data.Projects, project)
	app.logActivityLocked(user.ID, "", r, "PROJECT_CREATED", "Created project: "+name)
	app.saveLocked()
	app.mu.Unlock()

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
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
	projects := app.Data.Projects
	selectedID := strings.TrimSpace(r.URL.Query().Get("project"))
	project, _ := app.projectByIDLocked(selectedID)
	var tables []map[string]interface{}
	if project != nil {
		tables = tableSummaries(project.Tables)
		selectedID = project.ID
	}
	app.Render(w, "tables", map[string]interface{}{"User": user, "Projects": projects, "SelectedProjectID": selectedID, "Tables": tables, "Active": "tables"})
}

func (app *App) HandleSQL(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	data := map[string]interface{}{"User": user, "Active": "sql", "BaseURL": requestBaseURL(r)}
	if r.Method == http.MethodPost {
		projectID := strings.TrimSpace(r.FormValue("project_id"))
		query := strings.TrimSpace(r.FormValue("query"))
		result, errText := app.runSQLLikeQuery(user, r, projectID, query)
		data["SelectedProjectID"] = projectID
		data["Query"] = query
		data["Result"] = result
		data["Error"] = errText
	}
	app.mu.Lock()
	data["Projects"] = app.Data.Projects
	app.mu.Unlock()
	app.Render(w, "sql", data)
}

func (app *App) HandleFeatures(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.Render(w, "features", map[string]interface{}{"User": user, "FeatureGroups": platformFeatures(), "Active": "features"})
}

func (app *App) HandleAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := app.CurrentUser(r)
	app.mu.Lock()
	defer app.mu.Unlock()
	app.Render(w, "api_keys", map[string]interface{}{"User": user, "APIKeys": app.Data.APIKeys, "Projects": app.Data.Projects, "Active": "api_keys", "BaseURL": requestBaseURL(r)})
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
	projectID := strings.TrimSpace(r.FormValue("project_id"))
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

	app.mu.Lock()
	if projectID == "" {
		if project, ok := app.projectByIDLocked(""); ok {
			projectID = project.ID
		}
	}
	app.mu.Unlock()

	apiKey := APIKey{
		ID:          newID("key"),
		Name:        name,
		Key:         "fh_" + keyType + "_" + randomToken(32),
		Type:        keyType,
		Privileges:  privileges,
		CreatedAt:   now(),
		CreatedBy:   user.ID,
		Description: description,
		ProjectID:   projectID,
	}

	app.mu.Lock()
	app.Data.APIKeys = append(app.Data.APIKeys, apiKey)
	app.logActivityLocked(user.ID, apiKey.ID, r, "API_KEY_CREATED", "Created "+keyType+" API key: "+name)
	app.saveLocked()
	app.mu.Unlock()

	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

func (app *App) runSQLLikeQuery(user *User, r *http.Request, projectID, query string) (string, string) {
	if strings.TrimSpace(query) == "" {
		return "", "Enter a query first."
	}

	fields := strings.Fields(strings.TrimSuffix(query, ";"))
	if len(fields) == 0 {
		return "", "Enter a query first."
	}

	app.mu.Lock()
	defer app.mu.Unlock()
	project, ok := app.projectByIDLocked(projectID)
	if !ok {
		return "", "Project not found."
	}

	upper := strings.ToUpper(query)
	switch {
	case strings.HasPrefix(upper, "SHOW TABLES"):
		result, _ := json.MarshalIndent(tableSummaries(project.Tables), "", "  ")
		app.logActivityLocked(user.ID, "", r, "SQL_SHOW_TABLES", "Listed tables in "+project.Name)
		app.saveLocked()
		return string(result), ""
	case strings.HasPrefix(upper, "SELECT * FROM "):
		tableName := cleanTableName(strings.TrimSpace(query[len("SELECT * FROM "):]))
		records, exists := project.Tables[tableName]
		if !exists {
			return "", "Table not found: " + tableName
		}
		result, _ := json.MarshalIndent(records, "", "  ")
		app.logActivityLocked(user.ID, "", r, "SQL_SELECT", "Selected records from "+tableName)
		app.saveLocked()
		return string(result), ""
	case strings.HasPrefix(upper, "CREATE TABLE "):
		tableName := cleanTableName(strings.TrimSpace(query[len("CREATE TABLE "):]))
		if tableName == "" {
			return "", "Table name is required."
		}
		if _, exists := project.Tables[tableName]; !exists {
			project.Tables[tableName] = []interface{}{}
		}
		app.logActivityLocked(user.ID, "", r, "SQL_CREATE_TABLE", "Created table "+tableName+" in "+project.Name)
		app.saveLocked()
		return "Table ready: " + tableName, ""
	default:
		return "", "Supported commands: SHOW TABLES; SELECT * FROM table_name; CREATE TABLE table_name;"
	}
}
