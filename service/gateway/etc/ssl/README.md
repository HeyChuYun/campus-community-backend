# 证书配置说明

## 生成自签名证书（开发环境）

在当前目录下执行以下命令生成自签名证书：

```bash
# 生成私钥
openssl genrsa -out server.key 2048

# 生成证书签名请求
openssl req -new -key server.key -out server.csr

# 生成自签名证书（有效期365天）
openssl x509 -req -days 365 -in server.csr -signkey server.key -out server.crt
```

或者使用一条命令直接生成：

```bash
openssl req -x509 -newkey rsa:2048 -nodes -keyout server.key -out server.crt -days 365 -subj "/C=CN/ST=Beijing/L=Beijing/O=HFJJ/CN=localhost"
```

## 使用正式证书（生产环境）

1. 从证书颁发机构（如Let's Encrypt、阿里云、腾讯云）获取SSL证书
2. 将证书文件命名为 `server.crt`
3. 将私钥文件命名为 `server.key`
4. 放置在当前目录下

## 启用HTTPS

在 `gateway.yaml` 中取消注释以下配置：

```yaml
CertFile: etc/ssl/server.crt
KeyFile: etc/ssl/server.key
```

启用后，网关将同时支持HTTP和HTTPS访问：
- HTTP: http://your-domain:8080
- HTTPS: https://your-domain:8080

## 注意事项

1. **开发环境**：自签名证书会导致浏览器显示"不安全"警告，需要手动信任
2. **生产环境**：必须使用正式CA签发的证书
3. **证书安全**：
   - 私钥文件（server.key）权限应设置为 600
   - 不要将私钥提交到版本控制系统
   - 建议将 `*.key` 和 `*.crt` 添加到 `.gitignore`
4. **证书更新**：证书过期前需要及时更新，建议设置自动化续期（如使用certbot）
