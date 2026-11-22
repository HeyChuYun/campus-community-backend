# 第四步：RabbitMQ 事件设计

## 1. 目标

RabbitMQ 作为业务事件总线，负责解耦：

- `relation-service`
- `content-service`
- `im-service`

## 2. 为什么要用 MQ

不用 MQ 的问题：

- 点赞成功要同步等待通知发送完成
- 评论成功要依赖 IM 服务可用
- 服务间耦合重
- 高峰期容易互相拖垮

用了 MQ 后：

- 内容写入成功即可快速返回
- 通知异步投递
- 后续还能挂审核、分析、推荐消费者

## 3. Exchange 设计

建议统一一个 topic exchange：

```text
hfjj.biz.event
```

## 4. Routing Key 设计

建议按“域.对象.事件”命名：

### 4.1 content-service

- `content.post.created`
- `content.post.deleted`
- `content.post.liked`
- `content.post.unliked`
- `content.post.commented`
- `content.post.favorited`
- `content.post.unfavorited`

### 4.2 relation-service

- `relation.user.followed`
- `relation.user.unfollowed`
- `relation.user.blocked`
- `relation.user.unblocked`

## 5. Queue 设计

前期至少这一个队列：

```text
im.interaction.queue
```

绑定建议：

- `content.post.liked`
- `content.post.commented`
- `content.post.favorited`
- `relation.user.followed`

后续可扩展：

- `audit.content.queue`
- `analytics.biz.queue`

## 6. 事件消息体统一结构

建议统一格式：

```json
{
  "event_id": "01HTXYZABCDEFG",
  "event_type": "content.post.liked",
  "occurred_at": "2026-04-28T12:00:00+08:00",
  "producer": "content-service",
  "trace_id": "trace-xxx",
  "operator_user_id": 1001,
  "target_user_id": 2001,
  "resource_type": "post",
  "resource_id": 90001,
  "payload": {
    "post_id": 90001,
    "post_author_user_id": 2001
  }
}
```

## 7. 为什么必须有 event_id

RabbitMQ 消费一般按“至少一次”处理，重复投递必须可接受。

所以每条业务事件都需要：

- `event_id`

消费端基于 `event_id` 做幂等。

## 8. 生产端可靠性：Outbox Pattern

推荐每个业务服务都采用 outbox。

### 8.1 生产流程

以点赞为例：

1. 插入 `post_likes`
2. 插入 `outbox_events`
3. 同事务提交
4. dispatcher 扫描 `outbox_events`
5. 投递到 RabbitMQ
6. 更新 outbox 状态为已发送

### 8.2 这样能解决什么问题

避免：

- 数据库已成功
- MQ 事件没发出去

## 9. 消费端可靠性：Consume Log

`im-service` 消费时必须做消费日志。

流程：

1. 收到 MQ 事件
2. 先查 `event_consume_logs`
3. 已处理则跳过
4. 未处理则执行业务
5. 投递成功后记录消费日志

## 10. 失败与重试建议

### 10.1 生产失败

outbox dispatcher：

- `retry_count + 1`
- 设置 `next_retry_at`
- 达到上限后进入人工排查状态

### 10.2 消费失败

建议使用：

- 死信交换机
- 重试队列

前期可以先做简单重试，后期再上延迟重试队列。

## 11. im-service 消费处理示例

以 `content.post.liked` 为例：

1. 消费到事件
2. 判断是否是自己赞自己
3. 判断该事件是否需要通知
4. 拼装互动消息内容
5. 调 WuKongIM 投递
6. 写 `event_consume_logs`
7. 写 `im_message_audits`

## 12. 第一阶段建议先做什么

建议最小落地：

1. RabbitMQ 基础连接
2. `content-service` outbox 表
3. `relation-service` outbox 表
4. `im.interaction.queue`
5. `im-service` 幂等消费

## 13. 后续扩展方向

后期同一套 MQ 还可以继续挂：

- 审核服务
- 推荐服务
- 数据统计服务
- 搜索索引服务
