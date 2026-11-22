# 合经社区 · 后端（campus-community-backend）

合肥经济学院校园社区 App 的后端：**go-zero 微服务单体仓库**，包含网关与 7 个业务服务，使用 gRPC + Protobuf 通信。

> **开源说明**：本仓库是从私有仓库导出的**脱敏快照**。所有生产密钥、校内地址、对象存储凭据均已替换为占位符，历史提交未公开。请勿直接用于生产，部署前务必替换 `CHANGE_ME`。

## 架构

```
                    ┌──────────────┐
   App / Admin ───► │   gateway    │  HTTP + WebSocket
                    └──────┬───────┘
        ┌──────────┬───────┼────────┬──────────┬──────────┐
        ▼          ▼       ▼        ▼          ▼          ▼
   passport    school   relation  content      im      update
   (认证/RBAC) (教务)   (关注/拉黑) (Feed/审核) (私信)  (版本)
        │          │       │        │          │          │
        └──────────┴───────┴────────┴──────────┴──────────┘
                     gRPC / Protobuf
        PostgreSQL · Redis · RabbitMQ · etcd · MinIO
```

- **8 个服务**：gateway、passport、content、im、relation、school、update、captcha-solver
- **62 个 RPC**、7 份 proto 契约；`go.work` 管理 10 个模块
- **配置**：本地文件 + etcd 双配置源；etcd 同时做服务发现
- **数据**：PostgreSQL（36 个迁移脚本）、Redis、RabbitMQ（事务性 Outbox）、MinIO
- **认证**：JWT 双 Token、Casbin RBAC、设备指纹、管理端请求签名
- **实时**：自研 RFC 6455 WebSocket 网关 + 悟空 IM 接入
- **内容安全**：敏感词 / 百度内容审核 + 审核队列

## 目录

```
service/           8 个服务（每个服务独立 go.mod）
  gateway/         HTTP 网关、WebSocket、鉴权、限流、支付
  passport/        注册登录、JWT、Casbin
  content/         Feed、评论、话题、审核
  im/              私信、会话、通知
  relation/        关注、拉黑、关系计数
  school/          教务授权、课表、成绩
  update/          App 版本
  captcha-solver/  验证码识别
pkg/               缓存、配置、Outbox、RPC 等共享库
proto/             gRPC 契约
config/            本地配置（占位符）
deploy/            Compose、etcd、SQL 迁移
docs/              API / RPC / 配置治理文档
```

## 快速开始

需要 Go 1.24+、PostgreSQL、Redis、etcd、RabbitMQ（可用 `deploy/compose` 起）。

```bash
make local          # 本地文件配置
make local-etcd     # 本地 etcd 配置
make test
```

Windows 使用 `make local-windows`。

生产使用 etcd：

```bash
export APP_ENV=prod
export CONFIG_SOURCE=etcd
export ETCD_ENDPOINTS=127.0.0.1:2379
export CONFIG_NAMESPACE=/hfjj/prod
make prod-up
```

## 配置

- `config/shared.local.yaml` + `config/services/*.yaml`：本地调试
- `deploy/config/shared.prod.yaml` + etcd：生产
- 所有密钥均为 `CHANGE_ME` / 空值，按 `deploy/env/app.env.example` 注入

## 安全

见 [SECURITY.md](SECURITY.md)。默认配置不可直接上线；数据库 / 对象存储 / 支付密钥请自行生成并妥善保管。

## License

MIT © 2026 HeyChuYun
