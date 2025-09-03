package main

import (
	"log"
	"os"

	"tesla-token-service/internal/config"
	"tesla-token-service/internal/database"
	"tesla-token-service/internal/handler"
	"tesla-token-service/internal/middleware"
	"tesla-token-service/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// 加载环境变量
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found")
	}

	// 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 设置日志级别
	if cfg.Debug {
		logrus.SetLevel(logrus.DebugLevel)
		gin.SetMode(gin.DebugMode)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化数据库
	db, err := database.Initialize(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// 初始化服务
	tokenService := service.NewTokenService(db, cfg.Tesla)
	
	// 初始化处理器
	tokenHandler := handler.NewTokenHandler(tokenService)

	// 设置 Gin 路由
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 健康检查
	r.GET("/health", handler.HealthCheck)

	// API 路由
	api := r.Group("/api/v1")
	
	// 公开接口 - 用于初始化 token
	api.POST("/tokens", tokenHandler.SaveToken)
	
	// 受保护的接口
	protected := api.Group("/tokens")
	protected.Use(middleware.JWTAuthMiddleware(cfg.JWT.Secret))
	{
		protected.GET("/:user_id", tokenHandler.GetToken)
		protected.GET("/:user_id/validate", tokenHandler.ValidateToken)
		protected.POST("/:user_id/refresh", tokenHandler.RefreshToken)
		protected.DELETE("/:user_id", tokenHandler.DeleteToken)
	}

	// 启动服务器
	addr := cfg.Server.Host + ":" + cfg.Server.Port
	logrus.Infof("Starting Tesla Token Service on %s", addr)
	
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}