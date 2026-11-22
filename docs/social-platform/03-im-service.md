# 第三步：im-service 设计

## 1. 目标

`im-service` 是你自己的聊天业务服务，不是替代 `WuKongIM`。

它的定位是：

- 平台消息业务层
- 私信规则层
- 互动通知投递层

## 2. 为什么必须有 im-service

因为 `WuKongIM` 不懂你的业务规则：

- 互关才算好友
- 单向关注只能发几条私信
- 被拉黑不能发消息
- 只读用户不能主动发消息
- 点赞和评论要发到互动消息栏

这些都必须由你自己的服务来做。

## 3. 服务职责

负责：

- 给前端签发 IM 连接凭证
- 映射平台用户和 IM 用户
- 私信权限校验
- 消费内容事件和关系事件
- 转换成互动消息
- 调用 WuKongIM 投递消息
- 消息审计
- 会话业务扩展

不负责：

- 自己实现 IM 长连接
- 自己实现消息投递协议

## 4. 私信规则建议

### 4.1 互关

- 可正常私信

### 4.2 单向关注

- 可发有限条消息
- 达到上限后必须等待对方回关

### 4.3 无关注关系

- 默认不可发

### 4.4 拉黑

- 被拉黑后不可发

### 4.5 平台权限受限

- 毕业只读、禁言、封禁后不可发

## 5. 推荐表结构

### 5.1 im_user_bindings

```sql
im_user_bindings
- user_id
- im_uid
- status
- created_at
- updated_at
```

### 5.2 im_dm_quotas

```sql
im_dm_quotas
- id
- sender_user_id
- receiver_user_id
- quota_date
- used_count
- limit_count
- created_at
- updated_at
```

唯一索引：

- `(sender_user_id, receiver_user_id, quota_date)`

### 5.3 im_message_audits

```sql
im_message_audits
- id
- message_id_in_wk
- conversation_type
- sender_user_id
- receiver_user_id
- biz_type
- content_preview
- status
- created_at
```

### 5.4 event_consume_logs

```sql
event_consume_logs
- id
- event_id
- consumer
- status
- consumed_at
```

唯一索引：

- `(event_id, consumer)`

## 6. 推荐接口

### 6.1 HTTP API

- `POST /api/v1/im/connect-token`
- `GET /api/v1/im/conversations`
- `GET /api/v1/im/interaction/messages`
- `POST /api/v1/im/dm/check`

### 6.2 gRPC 能力建议

```proto
service IMService {
  rpc IssueConnectToken(IssueConnectTokenRequest) returns (IssueConnectTokenResponse);
  rpc CheckDirectMessagePermission(CheckDirectMessagePermissionRequest) returns (CheckDirectMessagePermissionResponse);
  rpc PushInteractionMessage(PushInteractionMessageRequest) returns (PushInteractionMessageResponse);
}
```

## 7. 与 WuKongIM 的关系

### 7.1 正确分工

- `im-service` 决定能不能发
- `WuKongIM` 负责真正发出去

### 7.2 connect token

前端不能直接拿平台 token 当 IM token。

建议流程：

1. 前端带平台 `access_token`
2. 调 `POST /api/v1/im/connect-token`
3. `im-service` 返回：
   - `im_uid`
   - `im_token`
   - `im_ws_url`

## 8. 互动消息设计

你当前不单独拆 `notification-service`，那么互动通知统一由 `im-service` 投递到一个“互动会话”。

建议：

- 会话类型：`interaction`
- 会话 ID：`interaction:{target_user_id}`
- 发送者：系统用户，例如 `system_interaction`

消息体示例：

```json
{
  "biz_type": "interaction",
  "event_type": "content.post.liked",
  "operator_user_id": 1001,
  "operator_nickname": "张三",
  "resource_type": "post",
  "resource_id": 90001,
  "text": "赞了你的帖子"
}
```

## 9. MQ 消费职责

`im-service` 至少消费这些事件：

- `content.post.liked`
- `content.post.commented`
- `content.post.favorited`
- `relation.user.followed`

消费后：

1. 做幂等判断
2. 生成互动消息模型
3. 调 WuKongIM 投递
4. 写审计日志

## 10. 第一阶段建议先做什么

优先顺序：

1. `IssueConnectToken`
2. `CheckDirectMessagePermission`
3. `PushInteractionMessage`
4. RabbitMQ 消费者
5. WuKongIM 对接
