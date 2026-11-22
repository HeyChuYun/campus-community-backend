# content-service 可见性补充

## 1. 当前可见性范围

帖子当前只支持两种可见性：

- `所有人可见`
- `仅关注者可见`

对应协议字段：

- `POST_VISIBILITY_PUBLIC`
- `POST_VISIBILITY_FOLLOWERS_ONLY`

## 2. 发帖时如何保存

`content-service.CreatePost` 请求里已经加入：

- `visibility`

调用方发帖时直接传：

- `POST_VISIBILITY_PUBLIC`
- `POST_VISIBILITY_FOLLOWERS_ONLY`

如果传空值或未知值，当前骨架实现会自动归一化为：

- `POST_VISIBILITY_PUBLIC`

## 3. 查询时如何判断

### 3.1 作者本人

- 作者自己永远可见自己的帖子

### 3.2 所有人可见

- `POST_VISIBILITY_PUBLIC` 直接放行

### 3.3 仅关注者可见

- `POST_VISIBILITY_FOLLOWERS_ONLY` 要求查看者已经关注作者

建议查询步骤：

1. `content-service` 先查出候选帖子
2. 收集当前页涉及的作者 user_id
3. 批量调用 `relation-service.BatchGetRelation`
4. 读取 `is_following`
5. 过滤掉当前用户不可见的帖子

## 4. 为什么不放到 relation-service 处理

因为帖子可见性最终还是内容域规则。

职责边界应该是：

- `relation-service` 只回答“你和作者是什么关系”
- `content-service` 再根据这个关系决定“这条帖子你能不能看”

## 5. 后续扩展方向

后续如果要扩展，可以继续在 `PostVisibility` 中增加：

- `POST_VISIBILITY_SELF_ONLY`
- `POST_VISIBILITY_FRIENDS_ONLY`

但当前版本先只做两档：

- 公开
- 仅关注者可见
