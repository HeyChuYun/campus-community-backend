# 第一步：relation-service 设计

## 1. 目标

`relation-service` 负责“人与人”的关系，不负责“人与内容”的关系。

它是以下服务的基础依赖：

- `content-service`
- `im-service`

## 2. 服务职责

负责：

- 关注
- 取关
- 拉黑
- 取消拉黑
- 查询是否互关
- 查询是否好友
- 查询是否被拉黑
- 关系计数

不负责：

- 发帖
- 评论
- 点赞
- 收藏
- 私信投递

## 3. 业务规则

### 3.1 关注

- A 关注 B，写一条 `user_follows`
- 唯一键 `(follower_user_id, followee_user_id)`
- 重复关注应幂等

### 3.2 互关

- A 关注 B
- B 关注 A
- 则二者互关

### 3.3 好友

当前版本定义：

- 互关即好友

前期不单独维护 `friends` 表，直接由互关关系推导。

### 3.4 拉黑

- A 拉黑 B 后：
  - B 不能给 A 发私信
  - B 的内容可见性可受影响
  - A 和 B 的关系查询应优先返回 `blocked`

## 4. 推荐表结构

### 4.1 user_follows

```sql
user_follows
- id
- follower_user_id
- followee_user_id
- status
- created_at
- updated_at
```

索引建议：

- unique `(follower_user_id, followee_user_id)`
- index `(followee_user_id, status)`
- index `(follower_user_id, status)`

### 4.2 user_blocks

```sql
user_blocks
- id
- user_id
- blocked_user_id
- created_at
```

索引建议：

- unique `(user_id, blocked_user_id)`

### 4.3 user_relation_counters

```sql
user_relation_counters
- user_id
- following_count
- follower_count
- friend_count
- blocked_count
- updated_at
```

## 5. gRPC 能力建议

```proto
service RelationService {
  rpc Follow(FollowRequest) returns (FollowResponse);
  rpc Unfollow(UnfollowRequest) returns (UnfollowResponse);
  rpc BlockUser(BlockUserRequest) returns (BlockUserResponse);
  rpc UnblockUser(UnblockUserRequest) returns (UnblockUserResponse);
  rpc GetRelation(GetRelationRequest) returns (GetRelationResponse);
  rpc BatchGetRelation(BatchGetRelationRequest) returns (BatchGetRelationResponse);
  rpc ListFollowing(ListFollowingRequest) returns (ListFollowingResponse);
  rpc ListFollowers(ListFollowersRequest) returns (ListFollowersResponse);
}
```

## 6. 核心返回结构建议

`GetRelationResponse` 至少包含：

- `is_following`
- `is_followed_by`
- `is_mutual_follow`
- `is_friend`
- `is_blocking`
- `is_blocked_by`

## 7. 与其他服务的调用关系

### 7.1 content-service 调 relation-service

用于：

- 获取关注列表，构建关注流
- 查询是否被屏蔽
- 查询某些内容可见性

### 7.2 im-service 调 relation-service

用于：

- 判断是否互关
- 判断是否被拉黑
- 判断是否允许继续私信

## 8. 事件建议

`relation-service` 需要产出这些业务事件：

- `relation.user.followed`
- `relation.user.unfollowed`
- `relation.user.blocked`
- `relation.user.unblocked`

这些事件交给 `im-service` 消费后转成互动通知。

## 9. 第一阶段建议先做什么

先做最小闭环：

1. `Follow`
2. `Unfollow`
3. `BlockUser`
4. `UnblockUser`
5. `GetRelation`

其余列表接口可以第二阶段再补。
