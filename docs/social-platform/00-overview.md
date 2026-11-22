# 社区服务扩展总览

## 1. 目标

当前后端已经有：

- `gateway`
- `passport-service`
- `school-service`

下一阶段新增三块核心业务域：

- `relation-service`
- `content-service`
- `im-service`

并引入：

- `WuKongIM`
- `RabbitMQ`

## 2. 总体职责边界

### 2.1 gateway

负责：

- HTTP API 入口
- 统一鉴权
- 参数校验
- 统一响应
- 转发内部 gRPC

不负责：

- 业务编排细节
- MQ 事件生产消费
- IM 长连接

### 2.2 passport-service

负责：

- 平台注册
- 平台登录
- access token / refresh token
- 平台用户状态
- RBAC 鉴权

不负责：

- 内容
- 关系
- IM 聊天

### 2.3 school-service

负责：

- 校园授权
- 学校侧 session / TGT / cookie
- 教务类查询

不负责：

- 平台社交内容
- 平台聊天

### 2.4 relation-service

负责：

- 关注 / 取关
- 拉黑 / 取消拉黑
- 互关关系
- 好友判定
- 关系计数

### 2.5 content-service

负责：

- 发帖
- 评论
- 点赞
- 收藏
- 内容流
- 内容计数
- 内容事件生产

### 2.6 im-service

负责：

- IM 业务层
- 私信权限校验
- IM 凭证签发
- 互动通知投递
- 消费 MQ 事件并转发到 WuKongIM

### 2.7 WuKongIM

负责：

- 长连接
- 消息投递
- 会话
- 在线状态
- 未读基础能力

不负责：

- 互关判断
- 单向关注私信额度
- 拉黑规则
- 毕业后只读限制

### 2.8 RabbitMQ

负责：

- 业务事件总线
- 解耦内容域、关系域、IM 域

## 3. 推荐总体链路

### 3.1 点赞链路

1. 前端调用 `POST /api/v1/content/posts/{post_id}/like`
2. `gateway` 鉴权后转发给 `content-service`
3. `content-service` 落库成功
4. `content-service` 写 `outbox_events`
5. Outbox dispatcher 投递 `content.post.liked`
6. `im-service` 消费事件
7. `im-service` 生成互动消息
8. `im-service` 调用 `WuKongIM` 投递给目标用户

### 3.2 评论链路

1. 前端调用评论接口
2. `content-service` 写评论
3. 产出 `content.post.commented`
4. `im-service` 消费后转成互动消息

### 3.3 关注链路

1. 前端调用关注接口
2. `relation-service` 写关注关系
3. 产出 `relation.user.followed`
4. `im-service` 消费后转成互动消息

### 3.4 私信链路

1. 前端通过平台 token 调 `POST /api/v1/im/connect-token`
2. `im-service` 校验平台登录态
3. `im-service` 校验账号状态
4. `im-service` 返回 WuKongIM 连接凭证
5. 前端连接 WuKongIM
6. 发消息时由 `im-service` 做发送前权限判断或回调校验

## 4. 实施顺序

建议按这四步推进：

1. 先做 `relation-service`
2. 再做 `content-service`
3. 再接 `RabbitMQ + Outbox`
4. 最后做 `im-service + WuKongIM`

## 5. 文档索引

- [01-relation-service.md](E:/project/hfjj/backend/docs/social-platform/01-relation-service.md)
- [02-content-service.md](E:/project/hfjj/backend/docs/social-platform/02-content-service.md)
- [03-im-service.md](E:/project/hfjj/backend/docs/social-platform/03-im-service.md)
- [04-rabbitmq-event-design.md](E:/project/hfjj/backend/docs/social-platform/04-rabbitmq-event-design.md)
