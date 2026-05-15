# Backend Auth Service

A simple backend authentication service built in Golang demonstrating JWT authentication, role-based access control, and clean architecture.

## Tech Stack

- **Language:** Golang
- **Database:** SQLite
- **Authentication:** JWT (github.com/golang-jwt/jwt/v5)
- **Password Hashing:** bcrypt (golang.org/x/crypto/bcrypt)
- **HTTP Router:** net/http (standard library)

## Project Structure

```
backend-auth/
├── main.go              # Entry point
├── database/
│   └── db.go            # SQLite initialization
├── models/
│   └── user.go          # User struct and request models
├── handlers/
│   ├── auth.go          # Signup and Login handlers
│   └── user.go          # Profile and GetUsers handlers
├── middleware/
│   └── jwt.go           # JWT authentication middleware
└── utils/
    ├── jwt.go           # JWT token generation and validation
    └── password.go      # bcrypt password hashing
```

## Setup

```bash
# Install dependencies
go mod tidy

# Run the server
go run main.go
```

The server starts on `http://localhost:8080`.

## API Endpoints

| Method | Endpoint   | Auth Required | Description                     |
|--------|------------|---------------|---------------------------------|
| POST   | /signup    | No            | Register a new user             |
| POST   | /login     | No            | Login and receive JWT token     |
| GET    | /profile   | Yes           | Get logged-in user's profile    |
| GET    | /users     | Yes (Admin)   | Get all users (admin only)      |

## Sample Requests

### Signup

```bash
curl -X POST http://localhost:8080/signup \
  -H "Content-Type: application/json" \
  -d '{"name":"Jeevith","email":"jeevith@gmail.com","password":"password123","role":"user"}'
```

### Login

```bash
curl -X POST http://localhost:8080/login \
  -H "Content-Type: application/json" \
  -d '{"email":"jeevith@gmail.com","password":"password123"}'
```

### Profile (Protected)

```bash
curl http://localhost:8080/profile \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Users (Admin Only)

```bash
curl http://localhost:8080/users \
  -H "Authorization: Bearer ADMIN_JWT_TOKEN"
```

## Authorization

Protected routes require a JWT token in the `Authorization` header:

```
Authorization: Bearer <JWT_TOKEN>
```

Tokens are obtained by logging in via the `/login` endpoint. Tokens expire after 24 hours.

## Roles

- **user** — Can access their own profile (`/profile`)
- **admin** — Can access all users list (`/users`) and their own profile
