# SurrealQL Commands for WhatsADK

This document provides SurrealQL equivalents for common database queries used to inspect and manage a WhatsADK SurrealDB backend.

## 🚀 Running SurrealDB with Docker

Before spinning up Docker for SurrealDB, check if `surreal` is installed locally using the `surreal --version` command:

```bash
surreal --version
```

If it is not installed or you prefer running it isolated in Docker, start the SurrealDB container for local development:

```bash
docker run --name whatsadk-surreal -d -p 8000:8000 surrealdb/surrealdb:latest start --user root --pass rootpassword
```

---

## 👥 Extract Contacts

To export all synchronized WhatsApp contacts:

### Via Surreal SQL CLI
```bash
docker exec -i whatsadk-surreal surreal sql --endpoint http://localhost:8000 --ns whatsadk --db whatsadk --user root --pass rootpassword "SELECT * FROM whatsmeow_contacts;" > whatsapp-contacts.txt
```

### Via HTTP REST API
```bash
curl -X POST -u "root:rootpassword" \
  -H "Accept: application/json" \
  -H "NS: whatsadk" \
  -H "DB: whatsadk" \
  -d "SELECT * FROM whatsmeow_contacts;" \
  http://localhost:8000/sql >> whatsapp-contacts.txt
```

---

## 📋 List Tables (Schemas)

To inspect all defined schemas and tables in the database:

```bash
docker exec -i whatsadk-surreal surreal sql --endpoint http://localhost:8000 --ns whatsadk --db whatsadk --user root --pass rootpassword "INFO FOR DB;"
```

Output will include defined tables such as:
- `blacklisted_numbers`
- `filesys`
- `whatsmeow_commands`
- `whatsmeow_contacts`

---

## 📊 Record Counts and Queries (`filesys` Table)

### 1. Check if there are any records matching a specific number in the path

This query uses `CONTAINS` to search for paths matching a phone number (e.g. `7838534799`):

```bash
docker exec -i whatsadk-surreal surreal sql --endpoint http://localhost:8000 --ns whatsadk --db whatsadk --user root --pass rootpassword \
  "SELECT count() FROM filesys WHERE path CONTAINS '7838534799' GROUP ALL;"
```

### 2. List all unique phone numbers stored in the database

This query splits the path by `/` and gets the 2nd element (index 1), grouping by it:

```bash
docker exec -i whatsadk-surreal surreal sql --endpoint http://localhost:8000 --ns whatsadk --db whatsadk --user root --pass rootpassword \
  "SELECT string::split(path, '/')[1] AS user_id FROM filesys WHERE path CONTAINS 'whatsmeow/' GROUP BY user_id;"
```

### 3. Check the total number of records in the `filesys` table

```bash
docker exec -i whatsadk-surreal surreal sql --endpoint http://localhost:8000 --ns whatsadk --db whatsadk --user root --pass rootpassword \
  "SELECT count() FROM filesys GROUP ALL;"
```
