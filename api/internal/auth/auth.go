package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// Default cookie/token timings. Access tokens are short so an inactive session
// loses elevated privileges quickly; refresh tokens carry the durable identity
// and rotate every successful refresh.
const (
	AccessTokenTTL          = 15 * time.Minute
	RefreshTokenTTL         = 24 * time.Hour
	RefreshTokenTTLRemember = 30 * 24 * time.Hour
	AccessCookieName        = "relay_access"
	RefreshCookieName       = "relay_refresh"
)

var (
	ErrInvalidToken       = errors.New("invalid token")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

type AuthService struct {
	jwtSecret     []byte
	refreshSecret []byte
}

func NewService(jwtSecret, refreshSecret string) *AuthService {
	return &AuthService{
		jwtSecret:     []byte(jwtSecret),
		refreshSecret: []byte(refreshSecret),
	}
}

// AccessClaims is the small subject + expiry payload we sign as the access
// token. It deliberately omits anything that could go stale (username, roles)
// so that re-auth picks up changes without a sign-out.
type AccessClaims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func (s *AuthService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

func (s *AuthService) VerifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func (s *AuthService) IssueAccessToken(userID string, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(AccessTokenTTL)
	claims := AccessClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			Subject:   userID,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (s *AuthService) ParseAccessToken(token string) (string, error) {
	claims := &AccessClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil || !parsed.Valid {
		return "", ErrInvalidToken
	}
	return claims.UserID, nil
}

// IssueRefreshToken returns a new opaque refresh token and a deterministic
// hash to store. The plaintext is given to the client; the hash is what the
// server keeps so a leaked DB never reveals usable tokens.
func (s *AuthService) IssueRefreshToken() (plaintext, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	plaintext = hex.EncodeToString(buf)
	return plaintext, s.HashRefreshToken(plaintext), nil
}

// HashRefreshToken keys refresh tokens by HMAC-style hash with the refresh
// secret. Using SHA-256 over (secret || token) lets us look up by hash in O(1)
// while still binding the hash to the deployment's secret.
func (s *AuthService) HashRefreshToken(plaintext string) string {
	h := sha256.New()
	h.Write(s.refreshSecret)
	h.Write([]byte(plaintext))
	return hex.EncodeToString(h.Sum(nil))
}

// RefreshTTL picks the lifetime based on whether the user opted in to
// "remember me" at sign-in time.
func RefreshTTL(remember bool) time.Duration {
	if remember {
		return RefreshTokenTTLRemember
	}
	return RefreshTokenTTL
}

// userIDContextKey is unexported so request handlers can only set/read the
// authenticated user via the helpers in this package.
type userIDContextKey struct{}

func ContextWithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(userIDContextKey{}).(string)
	return v, ok && v != ""
}
