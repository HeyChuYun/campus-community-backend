# School Service RPC Design

## 目标

`school-service` 负责：

- 校园授权
- 校园账号绑定
- 校园凭据加密保存
- 学校侧 session 管理
- 学校信息查询

它不负责：

- 平台密码
- 平台登录
- 平台 JWT
- 发帖、评论、点赞等平台能力

## 核心设计原则

### 1. 不暴露学校内部细节

对外 RPC 不直接暴露：

- `TGT`
- `CASTGC`
- `cookie`
- `service ticket`

这些都应该是 `school-service` 内部实现细节。

### 2. 查询学校信息统一传 `user_id`

不建议让上层一直传学号，因为：

- 平台内部应该统一用 `user_id`
- 一个用户未来可能支持多学校或多个学校账号

### 3. 学校侧 session 失效时由 school-service 内部处理

处理策略：

- 优先用缓存 session
- session 失效则自动重登
- 重登成功后继续查询
- 重登失败则返回 `REAUTH_REQUIRED`

## RPC 列表

### PreAuthorize

作用：

- 打开校园授权页前先拿验证码和授权会话

### Authorize

作用：

- 用户输入学号、校园密码、验证码后发起学校登录
- 登录成功后生成 `school_auth_ticket`

### BindPlatformUser

作用：

- 注册成功后把平台 `user_id` 和学校授权绑定

### EnsureSession

作用：

- 检查学校侧 session 是否有效
- 必要时自动重登

### GetStudentProfile

作用：

- 获取个人信息

### GetStudentSchedule

作用：

- 获取课表

### GetStudentGrade

作用：

- 获取成绩

### LogoutSchool

作用：

- 清理学校侧缓存和 session
