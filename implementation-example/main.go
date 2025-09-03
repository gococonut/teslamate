package main

import (
    "log"
    "os"
    
    "github.com/gin-gonic/gin"
    "github.com/joho/godotenv"
    
    "teslamate-token-service/config"
    "teslamate-token-service/database"
    "teslamate-token-service/middleware"
    "teslamate-token-service/routes"
)

func main() {
    // 加载环境变量
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found")
    }
    
    // 初始化配置
    cfg := config.New()
    
    // 初始化数据库
    db, err := database.Initialize(cfg.DatabaseURL)
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }
    
    // 自动迁移数据库
    if err := database.Migrate(db); err != nil {
        log.Fatal("Failed to migrate database:", err)
    }
    
    // 设置 Gin 模式
    gin.SetMode(cfg.GinMode)
    
    // 创建 Gin 实例
    r := gin.Default()
    
    // 添加中间件
    r.Use(middleware.CORS())
    r.Use(middleware.RequestLogger())
    r.Use(middleware.ErrorHandler())
    
    // 设置路由
    routes.SetupRoutes(r, db, cfg)
    
    // 启动服务器
    port := os.Getenv("PORT")
    if port == "" {
        port = "8080"
    }
    
    log.Printf("Server starting on port %s", port)
    if err := r.Run(":" + port); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}