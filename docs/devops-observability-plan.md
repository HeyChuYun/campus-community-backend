# HFJJ DevOps 与可观测性落地方案

## 1. 当前基础设施判断

你现在已经有：

- PostgreSQL
- Redis
- RabbitMQ
- MinIO
- Prometheus

这已经足够支撑当前阶段的后端系统。  
现阶段 **不建议为了“像企业”而继续补 etcd、k8s、service mesh**。

原因：

- 你当前还是典型的中小规模业务架构，不是超大规模微服务集群
- 现在更缺的是“能发现问题、能定位问题、能稳定发布”，不是“更复杂的注册中心”
- 如果暂时没有大规模动态服务发现、配置中心、分布式调度诉求，`etcd` 不是优先项

## 2. 下一步优先级

Prometheus 已经装了，下一步按这个顺序补：

1. Grafana  
   作用：看监控图表、做业务大盘、看告警趋势

2. Alertmanager  
   作用：Prometheus 告警聚合、去重、分组、通知

3. Loki  
   作用：集中收集日志，按服务、实例、traceId、userId 检索

4. Promtail  
   作用：把应用日志、Nginx 日志、系统日志推送到 Loki

5. Exporters  
   作用：补齐基础监控数据

推荐至少加：

- `node-exporter`：主机 CPU、内存、磁盘、网络
- `postgres-exporter`：PostgreSQL 指标
- `redis-exporter`：Redis 指标
- `rabbitmq-exporter` 或 RabbitMQ 自带 metrics
- `blackbox-exporter`：接口可用性、端口探活、域名探活

## 3. 现阶段不必急着装的东西

以下不是现在第一优先级：

- etcd
- Kubernetes
- Jaeger / Tempo / SkyWalking
- Service Mesh
- ELK 全家桶

如果后面服务数量继续膨胀、排障越来越依赖调用链，再考虑：

- `Tempo`：比 Jaeger 更轻，适合和 Prometheus + Grafana + Loki 搭配

## 4. 推荐的最终组合

当前项目建议形成这一套：

- `Prometheus`：指标采集
- `Grafana`：监控展示
- `Alertmanager`：告警通知
- `Loki`：日志中心
- `Promtail`：日志采集
- `Node Exporter`：主机监控
- `Postgres Exporter`：数据库监控
- `Redis Exporter`：缓存监控
- `RabbitMQ Metrics/Exporter`：消息队列监控
- `Blackbox Exporter`：外部探活

这是一个很典型、够用、成本可控的企业级基础盘。

## 5. 监控要看什么

### 5.1 系统层

- CPU 使用率
- 内存使用率
- 磁盘剩余空间
- 磁盘 IO
- 网络带宽
- 机器负载

### 5.2 网关层

- QPS
- 接口响应时间 P50 / P95 / P99
- 4xx / 5xx 数量
- 登录成功率
- 校园授权换票成功率
- 第三方接口错误率

### 5.3 业务层

- 发帖成功量
- 评论成功量
- 举报提交量
- 举报处理时长
- 私信发送成功率
- 审核积压数量

### 5.4 中间件层

- PostgreSQL 连接数、慢查询、事务提交/回滚
- Redis 命中率、内存占用、连接数
- RabbitMQ 队列堆积、消费者数量、消息确认率
- MinIO 存储容量、对象数量、错误率

## 6. 日志怎么做

不要只靠控制台打印。建议统一结构化日志。

推荐日志字段：

- `timestamp`
- `level`
- `service`
- `instance`
- `traceId`
- `requestId`
- `userId`
- `uri`
- `method`
- `status`
- `latencyMs`
- `error`

建议拆三类日志：

- 访问日志
- 业务日志
- 错误日志

重点要求：

- 敏感信息脱敏
- 登录凭证、token、密码、cookie 不入日志
- 校园系统返回内容不要完整明文落日志

## 7. 告警怎么设

### P1 立即告警

- 网关不可用
- 登录服务不可用
- 数据库不可连接
- RabbitMQ 大面积堆积
- 5xx 错误率持续升高

### P2 高优先级

- CPU 持续过高
- 内存持续过高
- 磁盘不足 15%
- 接口 P95 响应时间明显升高
- 校园授权换票失败率过高

### P3 观察类

- 举报积压超时
- 某个消费者堆积增长
- 某些定时任务连续失败

## 8. 分支与发布建议

建议仓库策略：

- `main`：生产分支
- `develop`：集成分支
- `feature/*`：功能开发
- `hotfix/*`：线上紧急修复

推荐流程：

1. 新功能从 `develop` 拉 `feature/*`
2. 功能完成提 PR 合并回 `develop`
3. 测试通过后，从 `develop` 合并到 `main`
4. `main` 每次合并自动触发生产构建与发布

## 9. CI/CD 建议

### 9.1 CI 阶段

每次 PR / Push 执行：

- 代码格式检查
- 单元测试
- 基础集成测试
- 构建 Docker 镜像
- 漏洞扫描（后续可加）

### 9.2 CD 阶段

当前更建议：

- Docker
- Docker Compose
- GitHub Actions / Gitea Actions / Jenkins 任一即可

不要一开始就上 Kubernetes。  
先把“构建稳定 + 发布稳定 + 回滚简单”做扎实。

推荐发布步骤：

1. 构建镜像
2. 推送镜像仓库
3. SSH 到服务器拉取最新镜像
4. `docker compose up -d`
5. 健康检查
6. 失败自动回滚上一版本

## 10. 配置管理怎么做

你之前问的 `etc` 自动配置，本质上不要把环境配置写死在仓库里。

建议方式：

- 仓库保留 `*.example.yaml`
- 真实配置放服务器
- 敏感值走环境变量
- 启动时注入环境变量覆盖模板

推荐结构：

- `etc/app.example.yaml`
- `etc/app.dev.yaml`
- `etc/app.prod.yaml`
- `.env.example`

敏感配置不要直接提交：

- 数据库密码
- Redis 密码
- JWT 密钥
- MinIO 密钥
- 校园系统账号密码
- 第三方短信/邮件密钥

## 11. 现阶段最值得补的两件事

如果只做最有价值的事，先做这两个：

1. 把 `Grafana + Loki + Promtail + Alertmanager` 补齐
2. 把 `develop -> main -> 自动发布` 这条流水线打通

这样你马上就能得到：

- 出问题能看图
- 出问题能查日志
- 出问题能收到告警
- 发版不用手工反复操作

## 12. 对你当前项目的结论

你的项目现在 **不缺 etcd**。  
你现在缺的是：

- 日志中心
- 告警通知
- 自动发布
- 环境配置标准化

先把这四个补完，才算真正进入 DevOps 阶段。

---

如果继续落地，下一步建议直接补这几份文件：

- `docker-compose.observability.yml`
- `deploy/prometheus/prometheus.yml`
- `deploy/alertmanager/alertmanager.yml`
- `deploy/loki/loki-config.yml`
- `deploy/promtail/promtail-config.yml`
- `deploy/grafana/provisioning/...`
- `.github/workflows/backend-ci.yml`
- `.github/workflows/backend-cd.yml`
