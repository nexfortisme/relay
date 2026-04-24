package app

import (
	"fmt"
	"log/slog"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/chat"
	"github.com/nexfortisme/relay/internal/config"
	"github.com/nexfortisme/relay/internal/httpapi"
	"github.com/nexfortisme/relay/internal/skills"
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

	st, err := store.New(cfg.SQLitePath)
	if err != nil {
		return nil, nil, err
	}

	_ = skills.NewStaticRegistry([]skills.Descriptor{
		{
			Name:              "default-chat",
			Description:       "Default chat behavior without external tools",
			Trigger:           "always",
			RequiredTools:     []string{},
			SystemPromptPatch: "",
		},
	})
	toolRuntime := tools.NewCompositeRuntime(
		tools.NewMCPRuntime([]tools.Definition{}),
		tools.NoopRuntime{},
	)
	chatService := chat.NewService(
		st,
		cfg.LLMURL,
		cfg.LLMModel,
		toolRuntime,
		logger,
		cfg.RelayDir,
		attachments.PromptOptions{
			MaxImageBytes: cfg.MaxImageBytes,
		},
	)
	handlers := httpapi.NewHandlers(chatService, logger, cfg.MaxUploadBytes)

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
		api.GET("/settings", handlers.GetSettings)
		api.PUT("/settings", handlers.UpdateSettings)
		api.POST("/conversations", handlers.CreateConversation)
		api.GET("/conversations", handlers.ListConversations)
		api.PATCH("/conversations/:id", handlers.RenameConversation)
		api.PATCH("/conversations/:id/archive", handlers.ArchiveConversation)
		api.PATCH("/conversations/:id/restore", handlers.RestoreConversation)
		api.DELETE("/conversations/:id", handlers.DeleteConversation)
		api.POST("/conversations/:id/stop", handlers.StopConversationGeneration)
		api.GET("/conversations/:id/messages", handlers.ListMessages)
		api.POST("/conversations/:id/messages", handlers.CreateMessage)
		api.POST("/conversations/:id/messages/failed", handlers.CreateFailedMessage)
		api.GET("/conversations/:id/messages/:messageId/attachments/:attachmentIndex/download", handlers.DownloadMessageAttachment)
		api.GET("/conversations/:id/stream", handlers.StreamConversation)
	}

	cleanup := func() {
		_ = st.Close()
	}

	logger.Info("server initialized", "port", cfg.Port, "sqlite_path", cfg.SQLitePath)
	return &Server{
		engine: engine,
		addr:   cfg.ListenAddr(),
	}, cleanup, nil
}
