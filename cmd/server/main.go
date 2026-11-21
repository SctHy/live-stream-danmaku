package main

import (
	"flag" // command line arguments
	"time"

	"github.com/charlesAcmen/livestream-danmaku/internal/api"
	"github.com/charlesAcmen/livestream-danmaku/internal/infra"
	"github.com/charlesAcmen/livestream-danmaku/internal/logger"
	"github.com/charlesAcmen/livestream-danmaku/internal/repo"
	"github.com/charlesAcmen/livestream-danmaku/internal/service"
	"github.com/charlesAcmen/livestream-danmaku/internal/ws"
	"github.com/gin-gonic/gin" // Gin is a web framework for Go.
	"go.uber.org/zap"
)

func main() {
	// 1. Initialize Logger
	// Use "dev" for colored output, "prod" for JSON.
	logger.InitLogger("dev")
	defer logger.Sync()

	// Define command-line flag for port. Default is 8080.
	// port is argument name,8080 is default value, "server port" is help message
	// port is in *string type
	port := flag.String("port", "8080", "server port")
	//resolve arguments,port is given value
	capacity := flag.Int("send-queue", ws.SendChanSize, "maximum frames buffered per connection")
	window := flag.Duration("batch-window", 5*time.Millisecond, "maximum hot-room coalescing delay; zero sends immediately")
	minViewers := flag.Int("batch-min-viewers", 128, "minimum batch-capable viewers per room")
	prepared := flag.Bool("prepared", false, "share WebSocket frames across recipients")
	flag.Parse()
	if *minViewers < 1 {
		logger.Log.Fatal("batch-min-viewers must be positive")
	}
	if *window < 0 {
		logger.Log.Fatal("batch-window must be nonnegative")
	}
	if *capacity < 1 {
		logger.Log.Fatal("send-queue must be positive")
	}
	logger.Log.Info("[SERVER]Starting server...", zap.String("port", *port))

	//IMPORTANT:
	//initialize Global WebSocket Handlers
	//logic handler functions[Danmaku,Like,Broadcast stats]
	ws.RegisterActionHandlers()

	// 1. Init DB (Shared by all layers)
	db := infra.InitDB()
	defer db.Close()
	// 2. Init Layers (Dependency Injection)
	// Repo -> Service -> Handler
	messageRepo := repo.NewMessageRepo(db)
	chatService := service.NewChatService(messageRepo)
	// 3. Init WebSocket Manager
	manager := ws.NewManager(*capacity)
	manager.MinBatchViewers = *minViewers
	manager.Prepared = *prepared
	manager.BatchWindow = *window
	go manager.Start()

	// 4. Setup API Handlers (Dependency Injection)
	// We pass the manager and service into the handler.
	chatHandler := api.NewChatHandler(chatService, manager)

	// 4. Init Router
	// r := gin.Default()
	//blank slate
	r := gin.New()
	// Manually attach Recovery middleware (Critical for preventing server crash on panic)
	r.Use(gin.Recovery())

	// WebSocket Route
	r.GET("/ws", chatHandler.ConnectWebSocket)

	addr := ":" + *port
	if err := r.Run(addr); err != nil {
		logger.Log.Fatal("[SERVER]Server startup failed", zap.Error(err))
	}
	logger.Log.Info("[SERVER]Server listening", zap.String("address", addr))
}
