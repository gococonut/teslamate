-- 初始化 Tesla Token 服务数据库

-- 创建 tesla_tokens 表
CREATE TABLE IF NOT EXISTS tesla_tokens (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) NOT NULL UNIQUE COMMENT '用户标识',
    access_token TEXT NOT NULL COMMENT '访问令牌',
    refresh_token TEXT NOT NULL COMMENT '刷新令牌',
    token_type VARCHAR(20) DEFAULT 'Bearer' COMMENT '令牌类型',
    expires_at TIMESTAMP NOT NULL COMMENT '访问令牌过期时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL,
    
    INDEX idx_user_id (user_id),
    INDEX idx_expires_at (expires_at),
    INDEX idx_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Tesla API Token 存储表';

-- 创建 token_usage_logs 表
CREATE TABLE IF NOT EXISTS token_usage_logs (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id VARCHAR(100) NOT NULL COMMENT '用户标识',
    action VARCHAR(50) NOT NULL COMMENT '操作类型: validate, refresh, create, delete',
    ip_address VARCHAR(45) COMMENT '客户端IP地址',
    user_agent TEXT COMMENT '用户代理',
    success BOOLEAN DEFAULT TRUE COMMENT '操作是否成功',
    error_message TEXT COMMENT '错误信息',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    
    INDEX idx_user_id (user_id),
    INDEX idx_created_at (created_at),
    INDEX idx_action (action)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Token 使用日志表';

-- 插入示例数据（可选，用于测试）
-- INSERT INTO tesla_tokens (user_id, access_token, refresh_token, expires_at) 
-- VALUES ('test_user', 'sample_access_token', 'sample_refresh_token', DATE_ADD(NOW(), INTERVAL 1 HOUR));