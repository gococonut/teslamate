package database

import (
	"fmt"
	"time"

	"tesla-token-service/internal/config"
	"tesla-token-service/internal/model"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Initialize 初始化数据库连接
func Initialize(cfg config.DatabaseConfig) (*gorm.DB, error) {
	// 构建 DSN
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Name,
	)

	// 配置 GORM 日志
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// 连接数据库
	db, err := gorm.Open(mysql.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	// 配置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %v", err)
	}

	// 设置连接池参数
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 自动迁移数据库结构
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %v", err)
	}

	logrus.Info("Database initialized successfully")
	return db, nil
}

// autoMigrate 自动迁移数据库结构
func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&model.TeslaToken{},
		&model.TokenUsageLog{},
	)
}

// CreateIndexes 创建数据库索引
func CreateIndexes(db *gorm.DB) error {
	// TeslaToken 表索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tesla_tokens_user_id ON tesla_tokens(user_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_tesla_tokens_expires_at ON tesla_tokens(expires_at)").Error; err != nil {
		return err
	}

	// TokenUsageLog 表索引
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_token_usage_logs_user_id ON token_usage_logs(user_id)").Error; err != nil {
		return err
	}
	if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_token_usage_logs_created_at ON token_usage_logs(created_at)").Error; err != nil {
		return err
	}

	logrus.Info("Database indexes created successfully")
	return nil
}