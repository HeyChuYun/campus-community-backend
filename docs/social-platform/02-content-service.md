# 第二步：content-service 设计

## 1. 目标

`content-service` 负责社区内容域，是后续信息流的核心服务。

## 2. 服务职责

负责：

- 发帖
- 删帖
- 评论
- 删评论
- 点赞
- 取消点赞
- 收藏
- 取消收藏
- 帖子详情
- 用户帖子列表
- 关注流
- 发现流
- 内容事件生产

不负责：

- 关注关系
- 私信
- IM 投递
- 登录

## 3. 发帖规则

建议把媒体规则放配置中心，不写死在代码里。

```yaml
Content:
  Media:
    MaxImageCount: 4
    MaxSingleImageBytes: 5242880
    MaxVideoBytes: 62914560
    MaxGifBytes: 62914560
```

规则：

- 纯文字允许
- 文字 + 图片允许
- 图片最多 4 张
- 单张图片不能超过 5MB
- 文字 + 视频允许
- 视频不能超过 60MB
- 文字 + GIF 允许
- GIF 不能超过 60MB
- 图片和视频不能同时存在
- 图片和 GIF 不能同时存在
- 视频和 GIF 不能同时存在

## 4. 权限前置判断

`content-service` 执行写操作前建议检查：

1. 平台 access token 是否有效
2. 用户是否被禁用
3. 用户是否处于只读状态
4. 用户是否被内容风控禁止发言

毕业后只读这种能力，不要在 `content-service` 内自己推导学校状态，而是直接读取平台权限快照。

## 5. 推荐表结构

### 5.1 posts

```sql
posts
- id
- author_user_id
- content_text
- content_type
- visibility
- status
- like_count
- comment_count
- favorite_count
- created_at
- updated_at
- deleted_at
```

### 5.2 post_media

```sql
post_media
- id
- post_id
- media_type
- storage_key
- url
- width
- height
- duration_ms
- size_bytes
- sort_order
- created_at
```

### 5.3 comments

```sql
comments
- id
- post_id
- user_id
- reply_to_comment_id
- reply_to_user_id
- content
- status
- created_at
- updated_at
- deleted_at
```

### 5.4 post_likes

```sql
post_likes
- id
- post_id
- user_id
- created_at
```

唯一索引：

- `(post_id, user_id)`

### 5.5 post_favorites

```sql
post_favorites
- id
- post_id
- user_id
- created_at
```

唯一索引：

- `(post_id, user_id)`

### 5.6 outbox_events

`content-service` 必须有自己的 outbox 表。

```sql
outbox_events
- id
- event_id
- event_type
- routing_key
- payload_json
- status
- retry_count
- next_retry_at
- created_at
- sent_at
```

## 6. 推荐接口

### 6.1 HTTP API

- `POST /api/v1/content/posts`
- `DELETE /api/v1/content/posts/{post_id}`
- `GET /api/v1/content/posts/{post_id}`
- `GET /api/v1/content/feed/following`
- `GET /api/v1/content/feed/discover`
- `POST /api/v1/content/comments`
- `DELETE /api/v1/content/comments/{comment_id}`
- `PUT /api/v1/content/posts/{post_id}/like`
- `DELETE /api/v1/content/posts/{post_id}/like`
- `PUT /api/v1/content/posts/{post_id}/favorite`
- `DELETE /api/v1/content/posts/{post_id}/favorite`

### 6.2 gRPC 能力建议

```proto
service ContentService {
  rpc CreatePost(CreatePostRequest) returns (CreatePostResponse);
  rpc DeletePost(DeletePostRequest) returns (DeletePostResponse);
  rpc GetPost(GetPostRequest) returns (GetPostResponse);
  rpc GetFollowingFeed(GetFollowingFeedRequest) returns (GetFollowingFeedResponse);
  rpc GetDiscoverFeed(GetDiscoverFeedRequest) returns (GetDiscoverFeedResponse);
  rpc CreateComment(CreateCommentRequest) returns (CreateCommentResponse);
  rpc DeleteComment(DeleteCommentRequest) returns (DeleteCommentResponse);
  rpc LikePost(LikePostRequest) returns (LikePostResponse);
  rpc UnlikePost(UnlikePostRequest) returns (UnlikePostResponse);
  rpc FavoritePost(FavoritePostRequest) returns (FavoritePostResponse);
  rpc UnfavoritePost(UnfavoritePostRequest) returns (UnfavoritePostResponse);
}
```

## 7. 时间线建议

### 7.1 following feed

逻辑：

1. 调 `relation-service` 获取关注用户列表
2. 查这些用户最近帖子
3. 按时间倒序分页

### 7.2 discover feed

前期不要做复杂推荐，建议：

- 按 `score DESC, created_at DESC`

`score` 可以先由简单热度计算得出：

- 点赞数
- 评论数
- 发布时间衰减

## 8. 内容事件

`content-service` 需要产出这些 MQ 事件：

- `content.post.created`
- `content.post.deleted`
- `content.post.liked`
- `content.post.unliked`
- `content.post.commented`
- `content.post.favorited`
- `content.post.unfavorited`

## 9. 与 im-service 的边界

`content-service` 只负责声明“发生了什么”，不负责“通知怎么发”。

例如：

- `content-service` 产出 `content.post.liked`
- `im-service` 决定：
  - 是否要通知
  - 通知给谁
  - 通过 WuKongIM 发成什么消息

## 10. 第一阶段建议先做什么

优先顺序：

1. 发帖
2. 删帖
3. 评论
4. 点赞
5. 收藏
6. 关注流
7. 事件 outbox
