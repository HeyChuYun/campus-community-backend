# 后续服务设计：内容、私信、权限

本文只做架构设计，不生成服务代码。当前后端仍保持 go-zero monorepo，后续新增服务继续放在 `backend/` 下，不拆仓库。

## 1. content-service

### 服务职责

`content-service` 负责社区内容域，不直接处理平台登录和校园登录。

- 发帖：支持文字、图片、视频、动图。
- 评论：支持楼中楼可后续扩展，第一版建议只做一级评论和回复目标。
- 点赞：帖子点赞、评论点赞。
- 收藏：用户收藏帖子。
- 内容状态：正常、仅自己可见、删除、审核中、封禁。
- 媒体规则校验：文件数量、大小、类型互斥。

### 边界

- 鉴权由 `gateway -> passport-service.VerifyAccessToken` 完成。
- 权限判断由后续 `permission-service` 完成，例如只读用户不能发帖、评论、点赞。
- 媒体文件本身建议进入对象存储，不直接存数据库。
- `content-service` 只保存媒体元数据和内容关系。

### 配置建议

```yaml
Content:
  Media:
    MaxImageCount: 4
    MaxSingleImageBytes: 5242880
    MaxVideoBytes: 62914560
    MaxGifBytes: 62914560
    AllowImageTypes:
      - image/jpeg
      - image/png
      - image/webp
    AllowVideoTypes:
      - video/mp4
    AllowGifTypes:
      - image/gif
```

### 发帖媒体规则

- 纯文字：允许。
- 文字 + 图片：允许，图片最多 4 张，单张最大 5M。
- 文字 + 视频：允许，视频最大 60M。
- 文字 + 动图：允许，动图最大 60M。
- 图片不能和视频同时出现。
- 图片不能和动图同时出现。
- 视频和动图不能同时出现。

### 建议 proto

```proto
service ContentService {
  rpc CreatePost(CreatePostRequest) returns (CreatePostResponse);
  rpc DeletePost(DeletePostRequest) returns (DeletePostResponse);
  rpc GetFeed(GetFeedRequest) returns (GetFeedResponse);
  rpc CreateComment(CreateCommentRequest) returns (CreateCommentResponse);
  rpc DeleteComment(DeleteCommentRequest) returns (DeleteCommentResponse);
  rpc LikeTarget(LikeTargetRequest) returns (LikeTargetResponse);
  rpc UnlikeTarget(UnlikeTargetRequest) returns (UnlikeTargetResponse);
  rpc FavoritePost(FavoritePostRequest) returns (FavoritePostResponse);
  rpc UnfavoritePost(UnfavoritePostRequest) returns (UnfavoritePostResponse);
}
```

### 建议表

- `posts`：帖子主表，保存作者、正文、状态、统计计数。
- `post_media`：帖子媒体表，保存对象存储 key、媒体类型、大小、宽高、封面。
- `comments`：评论表，保存帖子 ID、作者、回复目标、正文、状态。
- `likes`：点赞表，保存用户和目标对象，目标对象可为帖子或评论。
- `favorites`：收藏表，保存用户和帖子关系。

### 事件

`content-service` 产生互动事件，后续投递到消息域：

- `post_liked`
- `post_commented`
- `comment_liked`
- `comment_replied`

这些事件最终进入“互动消息”列表，而不是普通私信。

## 2. 私信服务：WuKongIM

### 服务职责

建议新增 `message-service` 作为业务侧 IM 适配层，底层使用 WuKongIM。

- 创建或查询单聊会话。
- 校验双方关系：互关是好友；单向关注只允许有限条私信。
- 给客户端签发 WuKongIM 连接所需 token。
- 同步或查询会话列表。
- 接收 WuKongIM webhook，落业务审计和风控记录。

### 边界

- 实时消息投递交给 WuKongIM。
- 好友关系由后续 `relation-service` 或用户关系模块维护。
- `message-service` 不直接做平台登录，只信任 gateway 传入的已认证 user_id。
- “点赞、评论、收藏”等互动消息不走私信会话，建议走 `notification-service` 或消息中心表。

### 建议接口

- `GetIMToken(user_id)`：获取 WuKongIM 连接 token。
- `CreateConversation(target_user_id)`：创建单聊。
- `SendMessage(conversation_id, payload)`：业务侧发送前置校验后调用 WuKongIM。
- `ListConversations(user_id)`：会话列表。
- `ListMessages(conversation_id)`：历史消息。

### 单向关注私信限制

规则建议放在配置中心：

```yaml
Message:
  FollowOnlyMaxMessages: 3
  RequireFriendAfterLimit: true
```

流程：

- 互相关注：正常私信。
- A 关注 B，但 B 未关注 A：A 最多给 B 发送配置数量的私信。
- 无关注关系：默认禁止私信，后续可改为允许一次打招呼。

## 3. permission-service：RBAC 权限服务

### 服务职责

`permission-service` 负责角色、权限、授权关系，不负责登录。

- 默认角色：`user`、`admin`。
- 用户绑定角色。
- 角色绑定权限。
- gateway 或业务服务调用权限校验。
- 支持平台账号状态约束，例如只读用户不能发帖、评论、点赞。

### 为什么不是直接写死

第一版可以自己写轻量 RBAC，不急着上 Casbin。原因：

- 当前权限模型简单，默认只有 `user/admin`。
- 你的主要限制来自业务状态，例如毕业只读、封禁、注销，这些写成业务规则更直观。
- 后续如果出现多租户、资源级权限、复杂策略，再引入 Casbin 成本更合理。

### 建议权限点

- `content.post.create`
- `content.post.delete.own`
- `content.post.delete.any`
- `content.comment.create`
- `content.comment.delete.own`
- `content.comment.delete.any`
- `content.like.create`
- `content.favorite.create`
- `message.send`
- `admin.user.ban`
- `admin.content.moderate`

### 建议表

- `roles`：角色表，默认写入 `user`、`admin`。
- `permissions`：权限点表。
- `role_permissions`：角色权限关系表。
- `user_roles`：用户角色关系表。

### 校验流程

普通内容写操作建议这样走：

1. `gateway` 校验 `access_token`。
2. `gateway` 把 `user_id` 转发给业务服务。
3. `content-service` 调用 `permission-service.CheckPermission`。
4. `permission-service` 同时检查角色权限和账号状态。
5. 通过后执行业务写入。

### 默认角色

- `user`：普通用户，允许发帖、评论、点赞、收藏、私信。
- `admin`：管理员，拥有内容管理、用户管理、审核能力。

## 4. 推荐新增目录

```text
backend/
  content-service/
  message-service/
  permission-service/
  docs/
    service-expansion-design.md
  proto/
    content.proto
    message.proto
    permission.proto
```

第一阶段建议先实现 `permission-service` 的轻量 RBAC，因为它会影响 `content-service` 的发帖、评论、点赞控制。
