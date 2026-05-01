package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nexfortisme/relay/internal/auth"
	"github.com/nexfortisme/relay/internal/chat"
	"github.com/nexfortisme/relay/internal/store"
)

type AuthHandlers struct {
	store        *store.Store
	auth         *auth.Service
	chat         *chat.Service
	logger       *slog.Logger
	cookieDomain string
	cookieSecure bool
	disableAuth  bool
	disabledUser string
}

func NewAuthHandlers(st *store.Store, authSvc *auth.Service, chatSvc *chat.Service, logger *slog.Logger, cookieSecure, disableAuth bool, disabledUserID string) *AuthHandlers {
	return &AuthHandlers{
		store:        st,
		auth:         authSvc,
		chat:         chatSvc,
		logger:       logger,
		cookieSecure: cookieSecure,
		disableAuth:  disableAuth,
		disabledUser: disabledUserID,
	}
}

type registerRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	RememberMe bool   `json:"rememberMe"`
}

type meResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

const minPasswordLength = 4

func (h *AuthHandlers) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if username == "" || len(username) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be at least 2 characters"})
		return
	}
	if len(req.Password) < minPasswordLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "password must be at least 4 characters"})
		return
	}

	if _, err := h.store.GetUserByUsername(c.Request.Context(), username); err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "username already taken"})
		return
	} else if !errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	hash, err := h.auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	user, err := h.store.CreateUser(c.Request.Context(), uuid.NewString(), username, hash, time.Now())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err := h.store.CopyDefaultSettings(c.Request.Context(), user.ID, h.chat.DefaultSettings()); err != nil {
		h.logger.Warn("failed to seed user settings", "user_id", user.ID, "error", err)
	}

	if err := h.issueSession(c, user.ID, req.RememberMe); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, meResponse{ID: user.ID, Username: user.Username})
}

func (h *AuthHandlers) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	username := strings.ToLower(strings.TrimSpace(req.Username))
	user, err := h.store.GetUserByUsername(c.Request.Context(), username)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if !h.auth.VerifyPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid username or password"})
		return
	}
	if err := h.issueSession(c, user.ID, req.RememberMe); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, meResponse{ID: user.ID, Username: user.Username})
}

func (h *AuthHandlers) Logout(c *gin.Context) {
	if cookie, err := c.Cookie(auth.RefreshCookieName); err == nil && cookie != "" {
		hash := h.auth.HashRefreshToken(cookie)
		if sess, err := h.store.GetSessionByRefreshHash(c.Request.Context(), hash); err == nil {
			_ = h.store.RevokeSession(c.Request.Context(), sess.ID)
		}
	}
	h.clearAuthCookies(c)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandlers) Refresh(c *gin.Context) {
	userID, err := h.refreshTokens(c)
	if err != nil {
		h.clearAuthCookies(c)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
		return
	}
	user, err := h.store.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "session expired"})
		return
	}
	c.JSON(http.StatusOK, meResponse{ID: user.ID, Username: user.Username})
}

func (h *AuthHandlers) Me(c *gin.Context) {
	userID, ok := auth.UserIDFromContext(c.Request.Context())
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	user, err := h.store.GetUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, meResponse{ID: user.ID, Username: user.Username})
}

// issueSession is the only place that mints fresh access+refresh tokens, so
// the cookie format and timing rules are guaranteed to stay in sync between
// register, login, and refresh.
func (h *AuthHandlers) issueSession(c *gin.Context, userID string, rememberMe bool) error {
	now := time.Now()
	access, accessExp, err := h.auth.IssueAccessToken(userID, now)
	if err != nil {
		return err
	}
	refresh, refreshHash, err := h.auth.IssueRefreshToken()
	if err != nil {
		return err
	}
	refreshExp := now.Add(auth.RefreshTTL(rememberMe))
	if err := h.store.CreateSession(c.Request.Context(), uuid.NewString(), userID, refreshHash, refreshExp, rememberMe, now); err != nil {
		return err
	}
	h.setAuthCookies(c, access, accessExp, refresh, refreshExp, rememberMe)
	return nil
}

// refreshTokens validates the incoming refresh cookie, rotates it, and
// re-issues an access token. Rotation invalidates the old refresh value, so a
// stolen-and-replayed refresh cookie stops working as soon as the legitimate
// client refreshes once.
func (h *AuthHandlers) refreshTokens(c *gin.Context) (string, error) {
	cookie, err := c.Cookie(auth.RefreshCookieName)
	if err != nil || cookie == "" {
		return "", errors.New("missing refresh")
	}
	hash := h.auth.HashRefreshToken(cookie)
	sess, err := h.store.GetSessionByRefreshHash(c.Request.Context(), hash)
	if err != nil {
		return "", err
	}
	if sess.RevokedAt != nil || time.Now().After(sess.ExpiresAt) {
		return "", errors.New("session expired")
	}

	now := time.Now()
	newRefresh, newHash, err := h.auth.IssueRefreshToken()
	if err != nil {
		return "", err
	}
	newRefreshExp := now.Add(auth.RefreshTTL(sess.RememberMe))
	if err := h.store.RotateSession(c.Request.Context(), sess.ID, newHash, newRefreshExp); err != nil {
		return "", err
	}
	access, accessExp, err := h.auth.IssueAccessToken(sess.UserID, now)
	if err != nil {
		return "", err
	}
	h.setAuthCookies(c, access, accessExp, newRefresh, newRefreshExp, sess.RememberMe)
	return sess.UserID, nil
}

func (h *AuthHandlers) setAuthCookies(c *gin.Context, access string, accessExp time.Time, refresh string, refreshExp time.Time, rememberMe bool) {
	// Access cookie always uses the access TTL — it is replaced on every
	// refresh anyway. Refresh cookie respects "remember me".
	accessMaxAge := int(time.Until(accessExp).Seconds())
	if accessMaxAge < 0 {
		accessMaxAge = 0
	}
	refreshMaxAge := 0 // session cookie when not remembering
	if rememberMe {
		refreshMaxAge = int(time.Until(refreshExp).Seconds())
		if refreshMaxAge < 0 {
			refreshMaxAge = 0
		}
	}
	sameSite := http.SameSiteLaxMode
	c.SetSameSite(sameSite)
	c.SetCookie(auth.AccessCookieName, access, accessMaxAge, "/", "", h.cookieSecure, true)
	c.SetCookie(auth.RefreshCookieName, refresh, refreshMaxAge, "/", "", h.cookieSecure, true)
}

func (h *AuthHandlers) clearAuthCookies(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(auth.AccessCookieName, "", -1, "/", "", h.cookieSecure, true)
	c.SetCookie(auth.RefreshCookieName, "", -1, "/", "", h.cookieSecure, true)
}

// Middleware authenticates every request that reaches a protected route. It
// silently swaps an expired access token for a freshly-rotated one when a
// valid refresh cookie is present, so well-behaved clients never see a 401
// during normal operation.
func (h *AuthHandlers) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.disableAuth {
			c.Request = c.Request.WithContext(auth.ContextWithUserID(c.Request.Context(), h.disabledUser))
			c.Set("userID", h.disabledUser)
			c.Next()
			return
		}

		userID, ok := h.userIDFromRequest(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}
		c.Request = c.Request.WithContext(auth.ContextWithUserID(c.Request.Context(), userID))
		c.Set("userID", userID)
		c.Next()
	}
}

func (h *AuthHandlers) userIDFromRequest(c *gin.Context) (string, bool) {
	if access, err := c.Cookie(auth.AccessCookieName); err == nil && access != "" {
		if userID, err := h.auth.ParseAccessToken(access); err == nil {
			return userID, true
		}
	}
	// Access token missing or expired — try a transparent refresh so an
	// active session does not get bounced back to the login screen.
	userID, err := h.refreshTokens(c)
	if err != nil {
		return "", false
	}
	return userID, true
}

// userIDFromGin mirrors UserIDFromContext but reads the gin context, which is
// the convenient handle inside HTTP handlers.
func userIDFromGin(c *gin.Context) (string, bool) {
	if v, ok := c.Get("userID"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s, true
		}
	}
	return auth.UserIDFromContext(c.Request.Context())
}

// EnsureRootUser creates the default admin user on startup if it does not
// exist, so a fresh dev DB can always be logged into without a manual
// registration step. The credentials come from config; rotate them before
// deploying anywhere real.
func EnsureRootUser(ctx context.Context, st *store.Store, authSvc *auth.Service, chatSvc *chat.Service, username, password string, logger *slog.Logger) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		username = "root"
	}
	existing, err := st.GetUserByUsername(ctx, username)
	if err == nil {
		return existing.ID, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return "", err
	}
	hash, err := authSvc.HashPassword(password)
	if err != nil {
		return "", err
	}
	user, err := st.CreateUser(ctx, uuid.NewString(), username, hash, time.Now())
	if err != nil {
		return "", err
	}
	if err := st.CopyDefaultSettings(ctx, user.ID, chatSvc.DefaultSettings()); err != nil {
		logger.Warn("failed to seed root user settings", "error", err)
	}
	logger.Info("seeded default root user", "username", username)
	return user.ID, nil
}
