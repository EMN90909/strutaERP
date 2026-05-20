# Struta ERP

A small Go + HTML backend dashboard that starts with first-time superuser setup, then lets the owner create API keys, view users, inspect activity, browse a Supabase-style feature map, and expose a JSON table API for external sites.

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
- `/api-keys` creates SDK or anon keys.
- `/tables` lists tables created through the API.
- `/users` lists platform users.
- `/activity` shows dashboard and API activity.

## Key Types

- SDK key: `auth`, `storage`, `realtime`
- Anon key: `external_tables_create`, `external_tables_read`, `external_tables_update`, `external_tables_delete`

## API

Use either a bearer token:

```text
Authorization: Bearer YOUR_API_KEY
```

or an `apikey` query parameter.

Create a table:

```powershell
curl -X POST http://localhost:8080/api/v1/tables `
  -H "Authorization: Bearer YOUR_API_KEY" `
  -H "Content-Type: application/json" `
  -d '{"name":"funeral_requests"}'
```

Insert a record:

```powershell
curl -X POST http://localhost:8080/api/v1/tables/funeral_requests `
  -H "Authorization: Bearer YOUR_API_KEY" `
  -H "Content-Type: application/json" `
  -d '{"family_name":"Mwangi","service":"burial","status":"pending"}'
```

Read records:

```powershell
curl http://localhost:8080/api/v1/tables/funeral_requests `
  -H "Authorization: Bearer YOUR_API_KEY"
```

Data is stored in `platform_data.json`.

## Docker

Build and run:

```powershell
docker build -t struta-erp .
docker run --rm -p 8080:8080 -v ${PWD}/platform_data.json:/app/platform_data.json struta-erp
```

If `platform_data.json` does not exist yet, run without the volume first or create an empty JSON file after first launch.
