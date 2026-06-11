package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateUser(
	ctx context.Context,
	id, username, passwordHash string,
	now time.Time,
) (User, error) {

	// Query to create the user
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users(id, username, password_hash, created_at) VALUES(?, ?, ?, ?)`,
		id, username, passwordHash, now.UTC(),
	)
	if err != nil {
		return User{}, fmt.Errorf("Error Creating User: %w", err)
	}

	// No errors, return an object mathing the created user
	return User{ID: id, Username: username, PasswordHash: passwordHash, CreatedAt: now.UTC()}, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (User, error) {

	// Query to fetch the user by username
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE username = ?`, username)

	// User object for returning
	var u User

	// Scanning the rows and parsing it into the user object
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("Error Fetching User: %w", err)
	}

	// Return the user object and any errors
	return u, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (User, error) {

	// Query to fetch the user by ID
	row := s.db.QueryRowContext(ctx, `SELECT id, username, password_hash, created_at FROM users WHERE id = ?`, id)

	// User object for returning
	var u User

	// Scanning the rows and parsing it into the user object
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("Error Fetching User: %w", err)
	}

	// Return the user object and any errors
	return u, nil
}

func (s *Store) UpdateUserPassword(ctx context.Context, id, passwordHash string) error {

	// Query to update the user password
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, passwordHash, id)
	if err != nil {
		return fmt.Errorf("Error Updating User Password: %w", err)
	}

	// No errors, return nil
	return nil
}

func (s *Store) CreateSession(
	ctx context.Context,
	sessionId,
	userID,
	refreshHash string,
	expiresAt time.Time,
	rememberMe bool,
	now time.Time,
) error {

	// Query to create the session
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, refresh_token_hash, expires_at, remember_me, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
		sessionId, userID, refreshHash, expiresAt.UTC(), rememberMe, now.UTC(),
	)
	if err != nil {
		return fmt.Errorf("Error Creating Session: %w", err)
	}

	// No errors, return nil
	return nil
}

// GetSessionByRefreshHash fetches a session by refresh hash
// Used for validating the refresh token and checking if it has been revoked
func (s *Store) GetSessionByRefreshHash(ctx context.Context, refreshHash string) (Session, error) {

	// Fetching the session by refresh hash
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, refresh_token_hash, expires_at, remember_me, created_at, revoked_at
		FROM sessions 
		WHERE refresh_token_hash = ?`, refreshHash,
	)

	// Session object for returning
	var sess Session
	var revoked sql.NullTime

	// Scanning the rows and parsing it into the session object
	if err := row.Scan(&sess.ID, &sess.UserID, &sess.RefreshTokenHash, &sess.ExpiresAt, &sess.RememberMe, &sess.CreatedAt, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Session{}, ErrNotFound
		}
		return Session{}, fmt.Errorf("Error Fetching Session: %w", err)
	}

	// Boolean Check to see if the session has been revoked
	if revoked.Valid {
		sess.RevokedAt = &revoked.Time
	}

	// Return the session object and any errors
	return sess, nil
}

func (s *Store) RotateSession(ctx context.Context, id, newRefreshHash string, expiresAt time.Time) error {

	// Query to rotate the session
	_, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET refresh_token_hash = ?, expires_at = ? WHERE id = ?`,
		newRefreshHash, expiresAt.UTC(), id,
	)
	if err != nil {
		return fmt.Errorf("rotate session: %w", err)
	}

	// No errors, return nil
	return nil
}

func (s *Store) RevokeSession(ctx context.Context, id string) error {

	// Query to revoke the session
	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE id = ?`, time.Now().UTC(), id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}

	// No errors, return nil
	return nil
}

// func (s *Store) RevokeUserSessions(ctx context.Context, userID string) error {

// 	// Query to revoke the user's sessions
// 	_, err := s.db.ExecContext(ctx, `UPDATE sessions SET revoked_at = ? WHERE user_id = ? AND revoked_at IS NULL`, time.Now().UTC(), userID)
// 	if err != nil {
// 		return fmt.Errorf("revoke user sessions: %w", err)
// 	}

// 	// No errors, return nil
// 	return nil
// }
