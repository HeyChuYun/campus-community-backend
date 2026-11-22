-- 审计日志表
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    action VARCHAR(50) NOT NULL,
    resource VARCHAR(100) NOT NULL,
    resource_id BIGINT,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(500) NOT NULL,
    ip VARCHAR(50) NOT NULL,
    user_agent VARCHAR(500),
    request_body TEXT,
    response_code INT NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- 创建索引
CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource, resource_id);

-- 添加注释
COMMENT ON TABLE audit_logs IS '审计日志表，记录所有敏感操作';
COMMENT ON COLUMN audit_logs.user_id IS '操作用户ID';
COMMENT ON COLUMN audit_logs.action IS '操作类型：create/update/delete/assign等';
COMMENT ON COLUMN audit_logs.resource IS '资源类型：post/comment/user等';
COMMENT ON COLUMN audit_logs.resource_id IS '资源ID';
COMMENT ON COLUMN audit_logs.method IS 'HTTP方法';
COMMENT ON COLUMN audit_logs.path IS '请求路径';
COMMENT ON COLUMN audit_logs.ip IS '客户端IP';
COMMENT ON COLUMN audit_logs.request_body IS '请求体（敏感字段已脱敏）';
COMMENT ON COLUMN audit_logs.response_code IS '响应状态码';
