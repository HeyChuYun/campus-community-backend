# Security Policy

## 这是一个脱敏快照

本仓库从私有仓库导出时已做以下处理：

- 删除 `deploy/env/app.env` 等包含部署参数的文件
- 生产 / 本地密钥替换为 `CHANGE_ME` 或空值
- 校内 VPN、教务系统地址、内网 IP 替换为 `example.edu` / `127.0.0.1`
- 对象存储地址与凭据替换为占位符
- 未公开任何历史提交

如果你在代码中发现仍残留的真实密钥或个人信息，请通过 GitHub Issue（不要公开内容）或邮件联系维护者，我会尽快处理。

## 部署前必须做

1. 生成新的 JWT secret、校园凭证主密钥、数据库密码、MinIO 凭据、支付密钥
2. 不要把 `deploy/env/app.env` 提交到任何仓库
3. RCON / PostgreSQL / Redis / MinIO 不要暴露公网
4. 生产环境开启 HTTPS，并限制管理端来源 IP

## 支持的版本

仅维护 `main` 分支的最新快照；本项目主要用于学习与作品展示，不承诺生产级 SLA。
