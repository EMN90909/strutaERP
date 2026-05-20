package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type App struct {
	mu          sync.Mutex
	Data        DataStore
	Sessions    map[string]string
	DataFile    string
	Templates   *template.Template
	TemplateMap map[string]*template.Template
}

type DataStore struct {
	Users      []User                   `json:"users"`
	APIKeys    []APIKey                 `json:"api_keys"`
	Activities []Activity               `json:"activities"`
	Tables     map[string][]interface{} `json:"tables"`
}

type User struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	PasswordHash string `json:"password_hash"`
	Role         string `json:"role"`
	CreatedAt    string `json:"created_at"`
}

type APIKey struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Key         string   `json:"key"`
	Type        string   `json:"type"`
	Privileges  []string `json:"privileges"`
	CreatedAt   string   `json:"created_at"`
	LastUsedAt  string   `json:"last_used_at"`
	CreatedBy   string   `json:"created_by"`
	Description string   `json:"description"`
}

type Activity struct {
	ID        string `json:"id"`
	Time      string `json:"time"`
	UserID    string `json:"user_id"`
	APIKeyID  string `json:"api_key_id"`
	Action    string `json:"action"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
	Details   string `json:"details"`
}

func NewApp(dataFile string) *App {
	funcs := template.FuncMap{
		"join": strings.Join,
	}

	pages := map[string]string{
		"setup":     "templates/setup.html",
		"login":     "templates/login.html",
		"dashboard": "templates/dashboard.html",
		"users":     "templates/users.html",
		"activity":  "templates/activity.html",
		"tables":    "templates/tables.html",
		"features":  "templates/features.html",
		"api_keys":  "templates/api_keys.html",
	}

	tm := map[string]*template.Template{}
	for name, page := range pages {
		tm[name] = template.Must(template.New("").Funcs(funcs).ParseFiles("templates/layout.html", page))
	}

	return &App{
		Sessions:    map[string]string{},
		DataFile:    dataFile,
		TemplateMap: tm,
	}
}

type FeatureGroup struct {
	Name     string
	Summary  string
	Features []string
}

func platformFeatures() []FeatureGroup {
	return []FeatureGroup{
		{
			Name:    "Database",
			Summary: "Postgres-style tables, SQL workflows, imports, relationships, extensions, backups, and realtime-enabled data.",
			Features: []string{
				"Managed database per project",
				"Table editor for creating and editing tables",
				"SQL editor for custom queries",
				"CSV and Excel import workflows",
				"Table cloning with data",
				"Relation and view exploration tools",
				"Extension-ready architecture for PostGIS, pgvector, and more",
				"Backups and restore points",
				"Realtime-enabled tables",
			},
		},
		{
			Name:    "Auth",
			Summary: "Identity, sessions, tokens, user management, roles, email flows, and row-aware authorization patterns.",
			Features: []string{
				"Email and password authentication",
				"Magic links, phone OTP, and passwordless login",
				"OAuth provider support",
				"Invite, block, and role management",
				"JWT sessions and refresh tokens",
				"RLS-aware access control patterns",
				"Email templates for invites, resets, and confirmations",
			},
		},
		{
			Name:    "Realtime",
			Summary: "WebSocket-style channels for database changes, presence, and broadcasts.",
			Features: []string{
				"Insert, update, and delete change listeners",
				"Presence for online state",
				"Broadcast messages for subscribed clients",
				"Postgres trigger and policy integration",
			},
		},
		{
			Name:    "Storage",
			Summary: "Object storage for files, images, blobs, analytics data, and vector workloads.",
			Features: []string{
				"S3-compatible object storage model",
				"CDN-ready public delivery",
				"Image transformation support",
				"General, analytics, and vector bucket types",
				"Row-level access patterns for files",
			},
		},
		{
			Name:    "Edge Functions",
			Summary: "Serverless HTTP functions for webhooks, background logic, and edge workflows.",
			Features: []string{
				"TypeScript and Deno-style function model",
				"Node.js compatible package usage",
				"Global execution strategy",
				"Logs, metrics, and tracing hooks",
				"HTTP triggers and webhooks",
			},
		},
		{
			Name:    "Data APIs",
			Summary: "Auto-generated REST, GraphQL, and realtime interfaces over project data.",
			Features: []string{
				"REST API from table schema",
				"GraphQL API with relationships",
				"Realtime WebSocket API for database changes",
			},
		},
		{
			Name:    "AI and Vectors",
			Summary: "Vector storage and semantic query patterns for AI-first apps.",
			Features: []string{
				"Vector embeddings storage",
				"Embedding generation workflows",
				"Semantic search and similarity queries",
				"AI toolkit integration patterns",
			},
		},
		{
			Name:    "Platform and Dev Tooling",
			Summary: "Local development, client libraries, dashboard operations, management APIs, scheduled jobs, and queues.",
			Features: []string{
				"Open-source core architecture",
				"CLI-driven local development",
				"Client libraries for major app platforms",
				"Dashboard settings, metrics, and logs",
				"Management API concepts",
				"Reusable UI components",
				"Cron and queues modules",
			},
		},
	}
}

func (app *App) Load() {
	app.mu.Lock()
	defer app.mu.Unlock()

	if _, err := os.Stat(app.DataFile); os.IsNotExist(err) {
		app.Data = emptyStore()
		app.saveLocked()
		return
	}

	b, err := os.ReadFile(app.DataFile)
	if err != nil {
		log.Fatal(err)
	}

	if len(strings.TrimSpace(string(b))) == 0 {
		app.Data = emptyStore()
		app.saveLocked()
		return
	}

	if err := json.Unmarshal(b, &app.Data); err != nil {
		log.Fatal(err)
	}
	if app.Data.Tables == nil {
		app.Data.Tables = map[string][]interface{}{}
	}
}

func emptyStore() DataStore {
	return DataStore{
		Users:      []User{},
		APIKeys:    []APIKey{},
		Activities: []Activity{},
		Tables:     map[string][]interface{}{},
	}
}

func (app *App) saveLocked() {
	b, _ := json.MarshalIndent(app.Data, "", "  ")
	_ = os.WriteFile(app.DataFile, b, 0600)
}

func (app *App) Render(w http.ResponseWriter, name string, data map[string]interface{}) {
	t, ok := app.TemplateMap[name]
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	if data == nil {
		data = map[string]interface{}{}
	}
	_ = t.ExecuteTemplate(w, "layout", data)
}

func (app *App) hasSuperuserLocked() bool {
	for _, u := range app.Data.Users {
		if u.Role == "superuser" {
			return true
		}
	}
	return false
}

func (app *App) CurrentUser(r *http.Request) *User {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}

	app.mu.Lock()
	defer app.mu.Unlock()

	userID, ok := app.Sessions[cookie.Value]
	if !ok {
		return nil
	}
	for i := range app.Data.Users {
		if app.Data.Users[i].ID == userID {
			return &app.Data.Users[i]
		}
	}
	return nil
}

func (app *App) RequireLogin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if app.CurrentUser(r) == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

func (app *App) createSession(w http.ResponseWriter, userID string) {
	token := randomToken(32)
	app.Sessions[token] = userID
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (app *App) logActivityLocked(userID, apiKeyID string, r *http.Request, action, details string) {
	app.Data.Activities = append(app.Data.Activities, Activity{
		ID:        newID("act"),
		Time:      now(),
		UserID:    userID,
		APIKeyID:  apiKeyID,
		Action:    action,
		IP:        r.RemoteAddr,
		UserAgent: r.UserAgent(),
		Details:   details,
	})
}

func (app *App) logAPIKeyUse(apiKeyID string, r *http.Request, action, details string) {
	app.mu.Lock()
	app.logActivityLocked("", apiKeyID, r, action, details)
	app.saveLocked()
	app.mu.Unlock()
}

func reverseActivities(items []Activity) []Activity {
	out := make([]Activity, len(items))
	copy(out, items)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > 50 {
		return out[:50]
	}
	return out
}

func tableSummaries(tables map[string][]interface{}) []map[string]interface{} {
	names := make([]string, 0, len(tables))
	for name := range tables {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		out = append(out, map[string]interface{}{
			"Name":  name,
			"Count": len(tables[name]),
		})
	}
	return out
}

func hasPrivilege(key *APIKey, privilege string) bool {
	for _, p := range key.Privileges {
		if p == privilege {
			return true
		}
	}
	return false
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte("localdb-platform-secret-salt::" + password))
	return hex.EncodeToString(sum[:])
}

func randomToken(size int) string {
	b := make([]byte, size)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func newID(prefix string) string {
	return prefix + "_" + randomToken(10)
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

func cleanTableName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	replacer := strings.NewReplacer(" ", "_", "/", "", "\\", "", ".", "", "-", "_")
	return replacer.Replace(name)
}

func readJSON(r *http.Request, dest interface{}) error {
	defer r.Body.Close()
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
