package main

import (
	"net/http"
)

func (app *App) HandleAPIStatus(w http.ResponseWriter, r *http.Request) {
	key, ok := app.authenticateAPIKey(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "Missing or invalid API key."})
		return
	}
	app.logAPIKeyUse(key.ID, r, "API_STATUS", "Checked API status")
	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "online", "key_type": key.Type, "privileges": key.Privileges})
}

func (app *App) HandleAPITables(w http.ResponseWriter, r *http.Request) {
	key, ok := app.authenticateAPIKey(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "Missing or invalid API key."})
		return
	}

	switch r.Method {
	case http.MethodGet:
		app.mu.Lock()
		tables := tableSummaries(app.Data.Tables)
		app.mu.Unlock()
		app.logAPIKeyUse(key.ID, r, "TABLES_LIST", "Listed tables")
		writeJSON(w, http.StatusOK, map[string]interface{}{"tables": tables})
	case http.MethodPost:
		if !hasPrivilege(key, "external_tables_create") && !hasPrivilege(key, "storage") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "API key does not have table creation privilege."})
			return
		}

		var payload struct {
			Name string `json:"name"`
		}
		if err := readJSON(r, &payload); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Invalid JSON."})
			return
		}

		tableName := cleanTableName(payload.Name)
		if tableName == "" {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Table name is required."})
			return
		}

		app.mu.Lock()
		if _, exists := app.Data.Tables[tableName]; !exists {
			app.Data.Tables[tableName] = []interface{}{}
		}
		app.logActivityLocked("", key.ID, r, "TABLE_CREATED", "Created table: "+tableName)
		app.saveLocked()
		app.mu.Unlock()

		writeJSON(w, http.StatusCreated, map[string]interface{}{"message": "Table created.", "table": tableName})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"error": "Method not allowed."})
	}
}

func (app *App) HandleAPITableRecords(w http.ResponseWriter, r *http.Request) {
	key, ok := app.authenticateAPIKey(r)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{"error": "Missing or invalid API key."})
		return
	}

	tableName := cleanTableName(r.URL.Path[len("/api/v1/tables/"):])
	if tableName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Table name is required."})
		return
	}

	switch r.Method {
	case http.MethodGet:
		if !hasPrivilege(key, "external_tables_read") && !hasPrivilege(key, "storage") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "API key does not have read privilege."})
			return
		}

		app.mu.Lock()
		records, exists := app.Data.Tables[tableName]
		app.mu.Unlock()
		if !exists {
			writeJSON(w, http.StatusNotFound, map[string]interface{}{"error": "Table not found."})
			return
		}

		app.logAPIKeyUse(key.ID, r, "TABLE_READ", "Read table: "+tableName)
		writeJSON(w, http.StatusOK, map[string]interface{}{"table": tableName, "records": records})
	case http.MethodPost:
		if !hasPrivilege(key, "external_tables_create") && !hasPrivilege(key, "storage") {
			writeJSON(w, http.StatusForbidden, map[string]interface{}{"error": "API key does not have insert privilege."})
			return
		}

		var record map[string]interface{}
		if err := readJSON(r, &record); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{"error": "Invalid JSON record."})
			return
		}
		record["id"] = newID("rec")
		record["created_at"] = now()

		app.mu.Lock()
		if _, exists := app.Data.Tables[tableName]; !exists {
			app.Data.Tables[tableName] = []interface{}{}
		}
		app.Data.Tables[tableName] = append(app.Data.Tables[tableName], record)
		app.logActivityLocked("", key.ID, r, "TABLE_RECORD_CREATED", "Inserted record into table: "+tableName)
		app.saveLocked()
		app.mu.Unlock()

		writeJSON(w, http.StatusCreated, map[string]interface{}{"message": "Record inserted.", "table": tableName, "record": record})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]interface{}{"error": "Method not allowed."})
	}
}

func (app *App) authenticateAPIKey(r *http.Request) (*APIKey, bool) {
	auth := r.Header.Get("Authorization")
	token := ""
	if len(auth) > 7 && auth[:7] == "Bearer " {
		token = auth[7:]
	}
	if token == "" {
		token = r.URL.Query().Get("apikey")
	}
	if token == "" {
		return nil, false
	}

	app.mu.Lock()
	defer app.mu.Unlock()
	for i := range app.Data.APIKeys {
		if app.Data.APIKeys[i].Key == token {
			app.Data.APIKeys[i].LastUsedAt = now()
			app.saveLocked()
			keyCopy := app.Data.APIKeys[i]
			return &keyCopy, true
		}
	}
	return nil, false
}
