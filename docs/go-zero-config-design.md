# Go-zero 配置设计方案

## 1. 目标

本方案面向当前 `backend/` 的 go-zero 多服务结构：

- `gateway`：HTTP 网关
- `passport-service`
- `school-service`
- `relation-service`
- `content-service`
- `im-service`
- `captcha-solver-service`
- `update-service`

设计目标：

- 本地测试可直接启动，不依赖额外渲染配置文件
- 生产部署统一走集中配置，适配容器化部署
- 配置结构和服务边界一致，避免一个大 yaml 承载所有环境
- 保持 go-zero 原生 `RestConf` / `RpcServerConf` / `RpcClientConf` 用法
- 敏感配置与业务配置分层管理，便于后续接入 CI/CD

## 2. 结论

建议采用两层配置模型：

1. 基础设施共享层
   - Postgres
   - Redis
   - RabbitMQ
   - Etcd 注册与发现

2. 服务业务配置层
   - JWT
   - 校园系统参数
   - 审核策略
   - 媒体存储
   - 商业化参数
   - 第三方密钥

运行时只保留两种来源：

- 本地测试：文件配置
- 生产部署：etcd 配置

不建议再引入：

- `.env` 作为业务运行配置载体
- `dynamic.yml`
- 渲染后的 `config/rendered/`

`.env` 只保留给 Docker Compose 这种“部署入口参数”使用，不承载业务配置本体。

## 3. 与当前项目的匹配关系

当前仓库已经基本具备这个方向：

- 有 `config/services/*.yaml`
- 有 `deploy/compose/docker-compose.app.yml`
- 有 `pkg/appconfig`
- 有 `docs/config-governance.md`

但现在存在一个需要统一的问题：

- 文档与 `config/services/*.yaml` 倾向“单服务配置”
- `pkg/appconfig/loader.go` 仍保留“`shared + services` 根配置合并”思路
- `pkg/cmd/localdev/main.go` 需要统一传入 `config/` 目录
- 仓库里实际存在的是 `config/shared.local.yaml` 和 `config/services/*.yaml`

所以最终应收敛为：

- 配置存储模型采用“共享基础配置 + 单服务配置”
- 运行时加载结果仍然是“每个服务拿到一份完整的 go-zero Config”
- 本地文件模式由加载器自动合并共享配置与服务配置
- 生产 etcd 模式按“共享 key + 服务 key”合并

## 4. 推荐目录设计

建议保持并明确如下目录：

```text
backend/
  config/
    shared.local.yaml
    services/
      gateway.yaml
      passport.yaml
      school.yaml
      relation.yaml
      content.yaml
      im.yaml
      captcha-solver.yaml
      update.yaml
    examples/
      shared.example.yaml
      gateway.example.yaml
      passport.example.yaml
      ...
  deploy/
    env/
      app.env
      app.env.example
    compose/
      docker-compose.app.yml
  docs/
    config-governance.md
    go-zero-config-design.md
```

说明：

- `shared.local.yaml`：本地基础设施默认值
- `services/*.yaml`：每个服务的本地业务配置
- `examples/*.example.yaml`：新环境初始化模板，不直接参与运行
- `app.env`：只提供部署入口变量，例如镜像 tag、etcd 地址、端口映射

## 5. 配置分层规范

### 5.1 共享配置

建议只放所有服务都会复用的内容：

```yaml
etcd:
  endpoints:
    - "127.0.0.1:2379"
  namespace: "/hfjj/local"

postgres:
  dsn: "postgres://..."

redis:
  host: "127.0.0.1:6379"
  type: "node"
  pass: ""

mq:
  enabled: false
  url: "amqp://guest:guest@127.0.0.1:5672/"
  exchange: "hfjj.biz.event"
  exchangeType: "topic"
  durable: true
```

放这里的标准：

- 3 个以上服务复用
- 变更时通常需要全局协同
- 不属于某个服务的独有业务含义

### 5.2 服务配置

只放单服务关注的业务配置，例如：

- `gateway.yaml`
  - `passportJwt`
  - `admin`
  - `mediaStorage`
  - `payment`
  - `commercial`
  - 下游 rpc client target

- `passport.yaml`
  - `token`
  - `geeTest`

- `school.yaml`
  - 校园登录地址
  - RSA 参数
  - 生命周期任务参数

- `content.yaml`
  - 发帖限制
  - 审核参数
  - outbox 参数

- `im.yaml`
  - 悟空 IM 地址
  - 互动消息 consumer

## 6. go-zero 配置结构建议

每个服务内部的 `internal/config/config.go` 保持现在这种方式：

- HTTP 服务嵌入 `rest.RestConf`
- RPC 服务嵌入 `zrpc.RpcServerConf`
- 调用下游服务使用 `zrpc.RpcClientConf`
- 业务字段放到服务自己的 `Config` struct

例如：

```go
type Config struct {
    rest.RestConf
    Postgres      PostgresConfig
    Redis         redis.RedisKeyConf
    PassportRpc   zrpc.RpcClientConf
    SchoolRpc     zrpc.RpcClientConf
    MediaStorage  MediaStorageConfig
}
```

这个方向是对的，建议继续保持。

关键约束：

- `Config` 结构只描述“本服务最终生效的完整配置”
- 不在服务层感知“这是来自 file 还是 etcd”
- 配置来源差异全部收敛在 `pkg/appconfig`

## 7. 运行时加载设计

### 7.1 本地测试模式

推荐环境变量：

```env
APP_ENV=local
CONFIG_SOURCE=file
SERVICE_NAME=content-service
CONFIG_DIR=./config
```

加载规则：

1. 读取 `config/shared.local.yaml`
2. 根据 `SERVICE_NAME` 映射读取 `config/services/content.yaml`
3. 合并共享层与服务层
4. 自动补齐 go-zero 启动字段
   - `Name`
   - `Host` / `Port`
   - `ListenOn`
   - `Etcd.Key`
   - `Telemetry.Name`
5. `conf.LoadFromYamlBytes` 加载到服务 `Config`

适合场景：

- 本地单机联调
- 本地跑集成测试
- Windows 开发机 / Linux 开发机

### 7.2 生产部署模式

推荐环境变量：

```env
APP_ENV=prod
CONFIG_SOURCE=etcd
ETCD_ENDPOINTS=127.0.0.1:2379
CONFIG_NAMESPACE=/hfjj
SERVICE_NAME=content-service
```

加载规则：

1. 从 etcd 读取共享配置
   - `/hfjj/prod/shared`
2. 从 etcd 读取服务配置
   - `/hfjj/prod/services/content`
3. 合并共享层与服务层
4. 自动补齐 go-zero 启动字段
5. 加载到服务 `Config`

这样设计的好处：

- 共享配置只维护一份
- 服务变更只更新自己的 key
- 生产环境不需要把完整业务配置打进镜像

## 8. etcd Key 规范

建议统一成下面这套：

```text
/hfjj/local/shared
/hfjj/local/services/gateway
/hfjj/local/services/passport
/hfjj/local/services/school
/hfjj/local/services/relation
/hfjj/local/services/content
/hfjj/local/services/im
/hfjj/local/services/captcha-solver
/hfjj/local/services/update

/hfjj/prod/shared
/hfjj/prod/services/gateway
/hfjj/prod/services/passport
/hfjj/prod/services/school
/hfjj/prod/services/relation
/hfjj/prod/services/content
/hfjj/prod/services/im
/hfjj/prod/services/captcha-solver
/hfjj/prod/services/update
```

不要把所有服务配置塞进一个总 key，原因：

- 任一服务修改都会影响整份大配置
- 回滚粒度太粗
- 权限控制不方便
- 审计 diff 不清晰

## 9. 本地测试方案

### 9.1 本地纯文件启动

推荐作为默认开发模式：

- `shared.local.yaml` 指向本机 Postgres / Redis / RabbitMQ / etcd
- `services/*.yaml` 填本地业务值
- `make local` 或 `go run ./pkg/cmd/localdev file`

适合：

- 日常开发
- 后端联调
- 排查业务逻辑问题

### 9.2 本地模拟生产

推荐增加一种“本地 etcd 模式”：

- `APP_ENV=local`
- `CONFIG_SOURCE=etcd`
- etcd 中写入 `/hfjj/local/shared` 和 `/hfjj/local/services/*`

适合：

- 验证配置中心逻辑
- 提前发现生产加载问题
- 部署脚本联调

### 9.3 测试环境

如果后续有独立测试环境，建议直接复用生产模型：

```text
/hfjj/test/shared
/hfjj/test/services/gateway
...
```

环境变量：

```env
APP_ENV=test
CONFIG_SOURCE=etcd
CONFIG_NAMESPACE=/hfjj
```

不要再单独设计第三套结构。

## 10. 生产部署方案

### 10.1 镜像职责

镜像只包含：

- 二进制
- 必要静态资源
- 启动命令

镜像不包含：

- 生产密码
- 生产 token secret
- 生产第三方密钥

### 10.2 Docker Compose / 容器入口参数

`deploy/env/app.env` 只放：

- `MODE` 或 `APP_ENV`
- `CONFIG_SOURCE`
- `ETCD_ENDPOINTS`
- `CONFIG_NAMESPACE`
- `SERVICE_NAME`
- 对外暴露端口
- 镜像仓库参数

例如：

```env
APP_ENV=prod
CONFIG_SOURCE=etcd
ETCD_ENDPOINTS=127.0.0.1:2379
CONFIG_NAMESPACE=/hfjj
IMAGE_TAG=v1.0.0
```

服务启动时只靠这些入口变量去 etcd 取配置。

### 10.3 发布流程建议

1. 先更新 etcd 配置
2. 配置通过校验
3. 再滚动发布服务镜像
4. 服务启动时输出配置版本和关键非敏感摘要

建议补充：

- 配置发布脚本
- 配置校验命令
- 配置回滚命令

## 11. 敏感信息治理

按敏感级别处理：

- 本地开发默认密钥
  - 可放 `shared.local.yaml` 或 `services/*.yaml`
- 测试环境真实密钥
  - 放 etcd
- 生产密钥
  - 放 etcd，限制读权限

高敏感字段包括：

- JWT secret
- 支付私钥
- MinIO secret
- 校园系统加密 key
- 第三方审核平台 AK/SK
- 管理员初始密码

要求：

- 文档中只出现示例值
- 代码日志里禁止完整打印
- 配置校验只检查“是否存在”，不打印原文

## 12. 校验与可观测性

每个服务保持 `Validate()` 是正确方向，建议继续强化：

- 基础字段必填校验
- URL / DSN / Duration 合法性校验
- 互斥字段校验
- feature 开启后的依赖校验

建议启动日志只打印这些非敏感信息：

- `service`
- `appEnv`
- `configSource`
- `listenOn` 或 `host:port`
- 下游 rpc target / etcd key
- 功能开关状态

不要打印：

- 密码
- token secret
- 私钥
- 第三方凭证

## 13. 推荐的统一环境变量

建议统一成下面这组，避免同时混用 `MODE`、`APP_ENV`、`ETCD_NAMESPACE`、`CONFIG_NAMESPACE` 等多套语义：

```env
APP_ENV=local|test|prod
CONFIG_SOURCE=file|etcd
CONFIG_NAMESPACE=/hfjj
ETCD_ENDPOINTS=127.0.0.1:2379
SERVICE_NAME=gateway|passport-service|...
CONFIG_DIR=./config
```

说明：

- `APP_ENV` 决定环境
- `CONFIG_SOURCE` 决定来源
- `CONFIG_NAMESPACE` 决定 etcd 根命名空间
- `SERVICE_NAME` 决定服务配置选择
- `CONFIG_DIR` 只在 file 模式有意义

不建议再让 `MODE=prod` 同时承担“环境”和“读取来源”两个职责。

## 14. 落地建议

### 第一阶段：统一模型

- 保留 `shared.local.yaml + services/*.yaml`
- 明确文档和代码都按这个模型走
- 修正 `pkg/appconfig` 的默认路径和加载规则

### 第二阶段：统一 etcd 结构

- 共享配置写入 `/hfjj/<env>/shared`
- 服务配置写入 `/hfjj/<env>/services/<service>`
- 补充 `etcdctl` 写入脚本

### 第三阶段：补充工具链

- 增加配置预校验命令
- 增加配置 diff 命令
- 增加本地 file / local-etcd / prod-etcd 三种启动入口

## 15. 最终建议

对你这个项目，最合适的 go-zero 配置方案不是“每个环境一份完整大 yaml”，而是：

- 共享基础设施配置一份
- 每个服务业务配置一份
- 本地开发走文件合并
- 生产部署走 etcd 合并
- 服务内部永远只接收一份完整的 go-zero `Config`

这样最符合你当前项目已经形成的多服务拆分、Docker 部署、etcd 运行时配置、以及 `go-zero` 原生配置结构。

