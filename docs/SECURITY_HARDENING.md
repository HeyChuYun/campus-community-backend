# 安全加固配置指南

## 1. 环境变量配置

创建 `.env` 文件（不要提交到Git）：

```bash
# 数据库配置
DB_HOST=localhost
DB_PORT=5432
DB_NAME=hfjj
DB_USER=hfjj_user
DB_PASSWORD=your_secure_password_here

# Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your_redis_password_here

# JWT密钥（使用强随机密钥）
JWT_ACCESS_SECRET=your_jwt_access_secret_min_32_chars
JWT_PREV_ACCESS_SECRET=your_previous_jwt_secret_for_rotation

# 校园密码加密主密钥
CAMPUS_MASTER_KEY=your_campus_encryption_key_min_32_chars

# 验证码服务
CAPTCHA_SOLVER_URL=http://localhost:18080/solve
CAPTCHA_FIXED_CODE=  # 仅开发环境使用

# CORS允许的域名（生产环境）
ALLOWED_ORIGINS=https://yourdomain.com,https://app.yourdomain.com

# 服务端口
GATEWAY_PORT=8080
PASSPORT_PORT=8081
CONTENT_PORT=8082
```

## 2. 生成安全密钥

```bash
# 生成JWT密钥
openssl rand -base64 32

# 生成校园密码加密密钥
openssl rand -base64 32
```

## 3. 数据库安全配置

```sql
-- 创建专用数据库用户
CREATE USER hfjj_user WITH PASSWORD 'your_secure_password';

-- 授予必要权限
GRANT CONNECT ON DATABASE hfjj TO hfjj_user;
GRANT USAGE ON SCHEMA public TO hfjj_user;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO hfjj_user;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO hfjj_user;

-- 限制连接数
ALTER USER hfjj_user CONNECTION LIMIT 50;
```

## 4. Redis安全配置

编辑 `redis.conf`：

```conf
# 绑定到本地
bind 127.0.0.1

# 设置密码
requirepass your_redis_password_here

# 禁用危险命令
rename-command FLUSHDB ""
rename-command FLUSHALL ""
rename-command CONFIG ""
rename-command SHUTDOWN ""

# 限制最大内存
maxmemory 2gb
maxmemory-policy allkeys-lru
```

## 5. Nginx配置（生产环境）

```nginx
# 限流配置
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
limit_conn_zone $binary_remote_addr zone=conn_limit:10m;

server {
    listen 443 ssl http2;
    server_name api.yourdomain.com;

    # SSL证书
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;

    # 安全头
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains; preload" always;
    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Content-Security-Policy "default-src 'self'" always;

    # 限流
    limit_req zone=api_limit burst=20 nodelay;
    limit_conn conn_limit 10;

    # 代理到后端
    location /api/ {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # 超时设置
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }

    # 禁止访问敏感文件
    location ~ /\. {
        deny all;
    }
}

# HTTP重定向到HTTPS
server {
    listen 80;
    server_name api.yourdomain.com;
    return 301 https://$server_name$request_uri;
}
```

## 6. 防火墙配置

```bash
# 仅允许必要端口
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw allow 22/tcp    # SSH
sudo ufw allow 80/tcp    # HTTP
sudo ufw allow 443/tcp   # HTTPS
sudo ufw enable

# 限制SSH登录尝试
sudo ufw limit 22/tcp
```

## 7. 应用配置更新

更新 `gateway/etc/config.yaml`：

```yaml
Name: gateway
Host: 0.0.0.0
Port: 8080

# 从环境变量读取敏感配置
Postgres:
  DSN: ${DB_DSN}

Redis:
  Host: ${REDIS_HOST}
  Type: node
  Pass: ${REDIS_PASSWORD}

PassportJwt:
  AccessSecret: ${JWT_ACCESS_SECRET}
  PrevAccessSecret: ${JWT_PREV_ACCESS_SECRET}

CampusCredential:
  MasterKey: ${CAMPUS_MASTER_KEY}
  TokenTTL: 2160h

# 限流配置
RateLimit:
  GlobalRPS: 100
  GlobalBurst: 200
  IPRPS: 20
  IPBurst: 40
  UserRPS: 50
  UserBurst: 100

# CORS配置
CORS:
  AllowedOrigins: ${ALLOWED_ORIGINS}

# 日志配置
Log:
  Mode: file
  Path: logs
  Level: info
  Compress: true
  KeepDays: 7
```

## 8. 密钥轮换流程

### JWT密钥轮换

1. 生成新密钥：`openssl rand -base64 32`
2. 将当前密钥设置为 `PrevAccessSecret`
3. 将新密钥设置为 `AccessSecret`
4. 重启服务
5. 等待旧token过期（默认15分钟）后移除 `PrevAccessSecret`

### 数据库密码轮换

1. 创建新密码
2. 更新数据库用户密码
3. 更新应用配置
4. 滚动重启服务

## 9. 监控告警

### 关键指标监控

- API响应时间 > 1s
- 错误率 > 1%
- 数据库连接数 > 80%
- Redis内存使用 > 80%
- 磁盘使用 > 80%
- 异常登录尝试（同IP 5分钟内失败>10次）

### 日志监控

```bash
# 监控错误日志
tail -f logs/error.log | grep -E "ERROR|FATAL"

# 监控审计日志
tail -f logs/audit.log | grep -E "delete|assign|ban"
```

## 10. 备份策略

### 数据库备份

```bash
# 每日全量备份
pg_dump -h localhost -U hfjj_user hfjj | gzip > backup_$(date +%Y%m%d).sql.gz

# 保留最近30天备份
find /backup -name "backup_*.sql.gz" -mtime +30 -delete
```

### Redis备份

```bash
# 配置RDB持久化
save 900 1
save 300 10
save 60 10000

# 配置AOF持久化
appendonly yes
appendfsync everysec
```

## 11. 安全检查清单

- [ ] 所有密钥使用环境变量，不硬编码
- [ ] 数据库使用专用用户，权限最小化
- [ ] Redis设置密码，禁用危险命令
- [ ] 启用HTTPS，配置HSTS
- [ ] 配置防火墙，仅开放必要端口
- [ ] 启用API限流
- [ ] 配置审计日志
- [ ] 定期备份数据库
- [ ] 配置监控告警
- [ ] 定期更新依赖包
- [ ] 定期进行安全扫描
- [ ] 制定应急响应预案
