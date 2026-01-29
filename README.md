# Lottery API

Small demo API for adding and listing lottery entries. Used as a lightweight target for APIdex drift detection (hackathon).

## APIs

| Method | Path      | Description        |
|--------|-----------|--------------------|
| GET    | /entries  | List all lottery entries |
| POST   | /entries  | Add a lottery entry (body: `{ "name": "...", "ticket": "..." }`) |

Responses are JSON. POST returns 201 with the created entry (id, name, ticket, created_at). Storage is in-memory (no database).

## Run

```bash
go mod tidy
go run .
```

Server listens on port 8080 by default (set `PORT` to use another port). Then:

- **Landing:** http://localhost:8080/ — click **Open Swagger UI**
- **Swagger UI:** http://localhost:8080/swagger — try GET /entries and POST /entries
- **OpenAPI spec:** http://localhost:8080/openapi.yaml

## Create a GitHub repo and push this project

Project path: `/home/era/lottery-api`.

### 1. Create the repo on GitHub

- Go to https://github.com/new
- **Repository name:** `lottery-api` (or any name you like)
- **Description:** optional, e.g. "Lottery API demo with OpenAPI spec and Swagger UI"
- Choose **Public**
- Do **not** check "Add a README", "Add .gitignore", or "Choose a license" (this project already has them)
- Click **Create repository**

### 2. Turn this folder into a git repo and push

In a terminal, from the project directory:

```bash
cd /home/era/ntnx-lottery

git init
git add .
git commit -m "Initial commit: Lottery API with OpenAPI spec and Swagger UI"
git branch -M main
git remote add origin https://github.com/YOUR_USERNAME/ntnx-lottery.git
git push -u origin main
```

Replace `YOUR_USERNAME` with your GitHub username (or your org name). If GitHub shows an SSH URL instead, use that:

```bash
git remote add origin git@github.com:YOUR_USERNAME/ntnx-lottery.git
git push -u origin main
```

### 3. If the repo already had a README (empty repo with README)

If you created the repo with "Add a README", you’ll need to pull and merge first:

```bash
cd /home/era/ntnx-lottery
git init
git add .
git commit -m "Initial commit: Lottery API with OpenAPI spec and Swagger UI"
git branch -M main
git remote add origin https://github.com/YOUR_USERNAME/ntnx-lottery.git
git pull origin main --allow-unrelated-histories
git push -u origin main
```
