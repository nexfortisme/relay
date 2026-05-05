package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/auth"
	"github.com/nexfortisme/relay/internal/chat"
	"github.com/nexfortisme/relay/internal/config"
	"github.com/nexfortisme/relay/internal/feeds"
	"github.com/nexfortisme/relay/internal/httpapi"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

type Server struct {
	engine *gin.Engine
	addr   string
}

func (s *Server) Run() error {
	return s.engine.Run(s.addr)
}

func NewServer(logger *slog.Logger) (*Server, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, fmt.Errorf("load config: %w", err)
	}
	return NewServerWithConfig(logger, cfg)
}

func NewServerWithConfig(logger *slog.Logger, cfg config.Config) (*Server, func(), error) {
	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		return nil, nil, err
	}

	toolRuntime := tools.NewCompositeRuntime(
		tools.NewMCPRuntime(cfg.MCPURL),
		tools.NoopRuntime{},
	)
	chatService := chat.NewService(
		st,
		cfg.LLMURL,
		cfg.LLMModel,
		toolRuntime,
		logger,
		attachments.PromptOptions{MaxImageBytes: cfg.MaxImageBytes},
		cfg.MaxTokenCount,
	)
	feedService := feeds.NewService(st, func(ctx context.Context, userID string) feeds.LLMSettings {
		settings := chatService.LoadRuntimeSettings(ctx, userID)
		return feeds.LLMSettings{
			LLMURL:    settings.LLMURL,
			LLMModel:  settings.LLMModel,
			LLMAPIKey: settings.LLMAPIKey,
		}
	}, logger)
	feedCtx, stopFeeds := context.WithCancel(context.Background())
	feedService.Start(feedCtx)

	handlers := httpapi.NewHandlers(chatService, feedService, logger, cfg.MaxUploadBytes)

	authSvc := auth.NewService(cfg.JWTSecret, cfg.JWTRefreshSecret)
	rootUserID, err := httpapi.EnsureRootUser(context.Background(), st, authSvc, chatService, cfg.RootUsername, cfg.RootPassword, logger)
	if err != nil {
		stopFeeds()
		_ = st.Close()
		return nil, nil, fmt.Errorf("seed root user: %w", err)
	}
	authHandlers := httpapi.NewAuthHandlers(st, authSvc, chatService, logger, cfg.CookieSecure, cfg.DisableAuth, rootUserID)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(httpapi.RequestID())
	engine.Use(httpapi.RequestLogger(logger))
	engine.Use(cors.New(cors.Config{
		AllowOrigins:     []string{cfg.WebOrigin},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
	}))

	engine.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	api := engine.Group("/api")
	{
		api.POST("/auth/register", authHandlers.Register)
		api.POST("/auth/login", authHandlers.Login)
		api.POST("/auth/logout", authHandlers.Logout)
		api.POST("/auth/refresh", authHandlers.Refresh)
	}

	authed := api.Group("")
	authed.Use(authHandlers.Middleware())
	{
		authed.GET("/auth/me", authHandlers.Me)
		authed.GET("/settings", handlers.GetSettings)
		authed.PUT("/settings", handlers.UpdateSettings)
		authed.POST("/conversations", handlers.CreateConversation)
		authed.GET("/conversations", handlers.ListConversations)
		authed.PATCH("/conversations/:id", handlers.RenameConversation)
		authed.POST("/conversations/:id/suggest-title", handlers.SuggestConversationTitle)
		authed.PATCH("/conversations/:id/archive", handlers.ArchiveConversation)
		authed.PATCH("/conversations/:id/restore", handlers.RestoreConversation)
		authed.DELETE("/conversations/:id", handlers.DeleteConversation)
		authed.POST("/conversations/:id/stop", handlers.StopConversationGeneration)
		authed.GET("/conversations/:id/messages", handlers.ListMessages)
		authed.POST("/conversations/:id/messages", handlers.CreateMessage)
		authed.POST("/conversations/:id/messages/failed", handlers.CreateFailedMessage)
		authed.POST("/conversations/:id/messages/:messageId/requeue", handlers.RequeueMessage)
		authed.GET("/files/:id/download", handlers.DownloadFile)
		authed.GET("/conversations/:id/stream", handlers.StreamConversation)
		authed.GET("/feeds", handlers.ListFeeds)
		authed.POST("/feeds/check", handlers.CheckFeed)
		authed.POST("/feeds", handlers.CreateFeed)
		authed.GET("/feeds/items", handlers.ListFeedItems)
		authed.GET("/feeds/items/:id", handlers.GetFeedItem)
		authed.PATCH("/feeds/items/:id", handlers.UpdateFeedItem)
		authed.POST("/feeds/items/:id/summarize", handlers.SummarizeFeedItem)
		authed.PATCH("/feeds/:id", handlers.UpdateFeed)
	}

	registerStaticWebUI(engine, logger)

	cleanup := func() {
		stopFeeds()
		_ = st.Close()
	}

	logger.Info("server initialized", "port", cfg.Port, "sqlite_path", cfg.SQLitePath, "mcp_url", cfg.MCPURL)
	return &Server{
		engine: engine,
		addr:   cfg.ListenAddr(),
	}, cleanup, nil
}

func registerStaticWebUI(engine *gin.Engine, logger *slog.Logger) {
	// Prefer a colocated frontend build output and quietly skip if absent (dev mode).
	distDir := filepath.Join("web", "dist")
	indexPath := filepath.Join(distDir, "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("unable to stat web dist index", "path", indexPath, "error", err)
		}
		return
	}

	engine.Static("/assets", filepath.Join(distDir, "assets"))
	engine.StaticFile("/favicon.ico", filepath.Join(distDir, "favicon.ico"))
	engine.StaticFile("/favicon.svg", filepath.Join(distDir, "favicon.svg"))
	engine.StaticFile("/apple-touch-icon.png", filepath.Join(distDir, "apple-touch-icon.png"))
	engine.StaticFile("/favicon-96x96.png", filepath.Join(distDir, "favicon-96x96.png"))
	engine.StaticFile("/site.webmanifest", filepath.Join(distDir, "site.webmanifest"))

	engine.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/health") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.File(indexPath)
	})
}
