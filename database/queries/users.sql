-- name: GetUsers :many
SELECT * FROM users;

-- name: GetUserById :one
SELECT * FROM users WHERE id = $1;

-- name: CreateUser :one
INSERT INTO users (name, email, password) VALUES ($1, $2, $3) RETURNING *;

-- name: UpdateUser :one
UPDATE users SET name = $1, email = $2, password = $3 WHERE id = $4 RETURNING *;

-- name: DeleteUser :one
DELETE FROM users WHERE id = $1 RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUsersCount :one
SELECT COUNT(*) FROM users;
