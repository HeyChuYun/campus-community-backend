# Passport RBAC 设计

## 1. 目标

在现有 `passport-service` 中直接扩展 RBAC 能力，而不是先拆一个独立 `permission-service`。

这样做的原因：

- 当前所有平台登录态都由 `passport-service` 管理。
- `gateway` 已经依赖 `passport-service.VerifyAccessToken` 做认证入口。
- 权限判断不仅是角色权限，还要结合账号状态，例如 `READ_ONLY`、`BLOCKED`、`DEACTIVATED`。

第一阶段目标：

- 保持 `passport-service` 既负责认证，也负责授权。
- 对外提供统一鉴权接口。
- 后续 `content-service`、`message-service`、管理后台都只调用 `passport-service` 做权限判断。

## 2. 职责边界

### 2.1 passport-service

负责：

- 平台账号注册、登录、刷新、退出。
- access_token / refresh_token 管理。
- 角色、权限点、用户角色关系管理。
- 账号状态与权限联合判断。
- 对外提供认证与授权 RPC。

### 2.2 school-service

不负责 RBAC。

仍然只负责：

- 校园授权。
- 学校侧 session / TGT / cookie 管理。
- 课表、成绩、个人信息查询。

`school-service` 只接收经过平台鉴权后的 `user_id`，不参与平台角色和权限判断。

## 3. 授权模型

最终授权结果由三层共同决定：

1. token 是否有效
2. 账号状态是否允许
3. 是否具备目标权限

即：

`允许访问 = access_token 有效 + account_status 允许 + permission 命中`

### 3.1 账号状态不是角色

以下状态不能建模为角色：

- `NORMAL`
- `READ_ONLY`
- `BLOCKED`
- `DEACTIVATED`

原因：

- 这些是全局账号状态，不是业务能力集合。
- 它们应该先于 RBAC 生效。
- 例如管理员角色的用户，只要账号是 `BLOCKED`，也不能继续操作。

### 3.2 RBAC 的职责

RBAC 只解决“用户有没有某项能力”，不解决“当前账号是不是还能执行写操作”。

例如：

- `admin` 拥有 `content.post.delete.any`
- `user` 拥有 `content.post.create`
- 但如果用户状态是 `READ_ONLY`，即使有 `content.post.create` 也必须拒绝

## 4. 数据表设计

保留现有 `users` 表，新增 4 张 RBAC 表。

### 4.1 roles

字段建议：

- `id bigserial primary key`
- `code varchar(64) not null unique`
- `name varchar(64) not null`
- `status smallint not null default 1`
- `is_system boolean not null default true`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

说明：

- `code` 作为角色稳定标识，例如 `user`、`admin`
- `is_system=true` 的角色表示系统内置角色，默认不允许随意删除

### 4.2 permissions

字段建议：

- `id bigserial primary key`
- `code varchar(128) not null unique`
- `name varchar(128) not null`
- `module varchar(64) not null`
- `description text not null default ''`
- `created_at timestamptz not null default now()`
- `updated_at timestamptz not null default now()`

说明：

- `code` 作为权限点稳定标识，例如 `content.post.create`
- `module` 便于后台按模块筛选

### 4.3 role_permissions

字段建议：

- `id bigserial primary key`
- `role_id bigint not null references roles(id) on delete cascade`
- `permission_id bigint not null references permissions(id) on delete cascade`
- `created_at timestamptz not null default now()`

约束建议：

- `unique(role_id, permission_id)`

### 4.4 user_roles

字段建议：

- `id bigserial primary key`
- `user_id bigint not null references users(id) on delete cascade`
- `role_id bigint not null references roles(id) on delete cascade`
- `created_at timestamptz not null default now()`

约束建议：

- `unique(user_id, role_id)`

## 5. 默认角色与权限

### 5.1 默认角色

系统初始化时写入：

- `user`
- `admin`

### 5.2 默认权限点

建议第一版至少内置这些权限点：

- `content.post.create`
- `content.post.delete.own`
- `content.post.delete.any`
- `content.comment.create`
- `content.comment.delete.own`
- `content.comment.delete.any`
- `content.like.create`
- `content.favorite.create`
- `message.send`
- `profile.update`
- `school.profile.read`
- `school.schedule.read`
- `school.grade.read`
- `admin.user.read`
- `admin.user.role.assign`
- `admin.user.ban`
- `admin.content.moderate`

### 5.3 默认角色权限关系

`user` 默认授予：

- `content.post.create`
- `content.post.delete.own`
- `content.comment.create`
- `content.comment.delete.own`
- `content.like.create`
- `content.favorite.create`
- `message.send`
- `profile.update`
- `school.profile.read`
- `school.schedule.read`
- `school.grade.read`

`admin` 默认授予：

- `user` 具备的全部权限
- `content.post.delete.any`
- `content.comment.delete.any`
- `admin.user.read`
- `admin.user.role.assign`
- `admin.user.ban`
- `admin.content.moderate`

## 6. 权限点命名规范

统一使用：

`{module}.{resource}.{action}`

例如：

- `content.post.create`
- `content.post.delete.own`
- `content.post.delete.any`
- `content.comment.create`
- `message.send`
- `school.profile.read`
- `admin.user.role.assign`

这样后续做：

- 网关接口鉴权
- 管理后台权限树
- 前端按钮权限
- 日志审计

都会更稳定。

## 7. 鉴权链路设计

### 7.1 认证

`gateway` 先调用：

- `passport-service.VerifyAccessToken`

用途：

- 校验 Bearer access_token 是否有效
- 拿到当前 `user_id`
- 拿到基础用户信息

### 7.2 授权

如果接口需要权限，再调用：

- `passport-service.Authorize`

用途：

- 校验 token 是否有效
- 校验账号状态是否允许
- 校验是否具备目标权限

### 7.3 示例

发帖：

1. `gateway` 校验 access_token
2. `gateway` 调 `Authorize(permission_code=content.post.create)`
3. 通过后调用 `content-service.CreatePost`

发送私信：

1. `gateway` 校验 access_token
2. `gateway` 调 `Authorize(permission_code=message.send)`
3. 通过后调用 `message-service.SendMessage`

后台分配角色：

1. `gateway` 校验 access_token
2. `gateway` 调 `Authorize(permission_code=admin.user.role.assign)`
3. 通过后调用 `passport-service.AssignUserRoles`

## 8. 账号状态与授权规则

### 8.1 NORMAL

- 正常按 RBAC 权限判断

### 8.2 READ_ONLY

- 允许读操作
- 禁止写操作

典型允许：

- 查看个人资料
- 查看学校资料
- 查看课表和成绩
- 查看内容流

典型拒绝：

- 发帖
- 评论
- 点赞
- 收藏
- 发送私信
- 修改角色

### 8.3 BLOCKED

- 禁止所有业务权限
- 可选允许登录查看封禁说明，但所有授权接口都返回拒绝

### 8.4 DEACTIVATED

- 拒绝所有授权
- 视为账号不可用

## 9. 缓存设计

不建议每次授权都查数据库。

建议在 Redis 中缓存用户授权快照：

- `authz:user:{user_id}`

缓存内容：

- `user_id`
- `account_status`
- `roles`
- `permissions`
- `updated_at`

刷新时机：

- 用户登录成功
- 分配角色后
- 角色权限变更后
- 账号状态变更后

授权流程建议：

1. 校验 access_token
2. 拿到 `user_id`
3. 优先读 `authz:user:{user_id}`
4. 未命中则查库并回填缓存
5. 执行状态 + 权限联合判断

## 10. proto 接口设计

本次 `passport.proto` 中新增：

- `Authorize`
- `GetMyAuthorization`
- `AssignUserRoles`
- `ListUserRoles`
- `ListRoles`
- `ListPermissions`

### 10.1 Authorize

入参：

- `access_token`
- `permission_code`
- `resource_owner_user_id`

返回：

- `allowed`
- `deny_reason`
- `snapshot`

说明：

- `resource_owner_user_id` 预留给资源级授权
- 第一版即使先不用，也建议保留字段

### 10.2 GetMyAuthorization

用途：

- 前端初始化后获取当前用户角色和权限快照
- 管理后台渲染菜单和按钮权限

### 10.3 AssignUserRoles

用途：

- 管理员给用户设置角色

第一版建议采用“全量替换目标用户角色”的语义，避免增删接口过多。

## 11. gateway 接入建议

第一版不建议把 RBAC 逻辑写在每个业务服务里。

建议方式：

- `gateway` 作为 HTTP 入口统一做 token 校验
- 对需要权限的接口，在 `gateway` 先调 `Authorize`
- 通过后再转发内部 RPC

这样可以保证：

- 鉴权入口统一
- 各业务服务不重复实现权限判断
- 后续权限规则调整只改 `passport-service`

## 12. 第一阶段实施顺序

建议按这个顺序推进：

1. 新增 RBAC 表结构
2. 初始化默认角色和权限点
3. 在 `passport-service` 增加角色、权限仓储层
4. 实现 `Authorize`
5. 实现 `GetMyAuthorization`
6. 实现 `AssignUserRoles`
7. 在 `gateway` 中给发帖、评论、点赞、收藏、私信等写接口接入授权检查

## 13. 第一阶段不做的内容

先不要做：

- 资源级复杂策略引擎
- `user_permissions` 直授权限
- 角色继承
- ABAC
- Casbin 集成
- 独立 `permission-service`

原因很简单：

- 当前模型还不复杂
- 过早拆服务只会增加 RPC 和缓存复杂度
- 先把认证和授权收口在 `passport-service`，最稳

## 14. 设计结论

第一版 RBAC 最优方案是：

- 直接并入 `passport-service`
- 用 RBAC 管角色和权限点
- 用 `account_status` 管只读、封禁、注销
- 用 `Authorize` 作为统一授权入口

这样后面：

- `content-service`
- `message-service`
- 管理后台

都可以复用同一套鉴权能力，而不需要重复造轮子。
