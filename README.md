# Struta ERP

A small Go + HTML backend dashboard that starts with first-time superuser setup, then lets the owner create projects, issue SDK credentials, inspect activity, browse a Supabase-style feature map, run SQL-style commands, and expose a JSON document table API for applications.

## Run

Install Go, then run:

```powershell
go run .
```

Open:

```text
http://localhost:8080
```

On the first deployment, the app redirects to `/setup` so you can create the superuser account. After that, sign in at `/login`.

## Pages

- `/dashboard` shows totals and an API quick example.
- `/api-keys` creates SDK or public anon SDK keys for a project.
- `/tables` lists document tables created through SDK writes.
- `/sql` runs lightweight SQL-style commands.
- `/users` lists platform users.
- `/activity` shows dashboard and API activity.

## SDK Types

- SDK: `auth`, `storage`, `realtime`
- Public anon SDK: `external_tables_create`, `external_tables_read`, `external_tables_update`, `external_tables_delete`

Each SDK key belongs to a project. API writes and reads are scoped to that project.

## Auto-created Tables

Tables are document-oriented and are created on first write. For example, the first `POST` to `/api/v1/tables/customers` stores the record and creates the `customers` table automatically if it does not already exist.

## API

Use either a bearer token:

```text
Authorization: Bearer YOUR_API_KEY
```

or an `apikey` query parameter.

Insert a record and auto-create the table:

```powershell
curl -X POST http://localhost:8080/api/v1/tables/customers `
  -H "Authorization: Bearer YOUR_API_KEY" `
  -H "Content-Type: application/json" `
  -d '{"name":"Acme Ltd","status":"lead"}'
```

Read records:

```powershell
curl http://localhost:8080/api/v1/tables/customers `
  -H "Authorization: Bearer YOUR_API_KEY"
```

## SQL Editor

Supported commands:

```sql
SHOW TABLES;
SELECT * FROM customers;
CREATE TABLE invoices;
```

Data is stored in `platform_data.json`.

## Docker

Build and run:

```powershell
docker build -t struta-erp .
docker run --rm -p 8080:8080 -v ${PWD}/platform_data.json:/app/platform_data.json struta-erp
```

If `platform_data.json` does not exist yet, run without the volume first or create an empty JSON file after first launch.
