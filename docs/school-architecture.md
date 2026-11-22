# School Architecture

## 目标

系统拆成两条账号体系：

- 平台账号：用于发帖、评论、点赞、收藏、私信、互动消息等平台内能力
- 校园账号：用于课表、成绩、个人信息等学校侧能力

两者通过 `user_id` 做绑定，但职责严格分层：

- `passport-service` 只负责平台账号注册、登录、JWT、账号状态
- `school-service` 只负责校园授权、校园凭据保存、TGT/session 管理、学校信息查询

## 推荐架构

```text
App
  -> gateway
      -> passport-service
      -> school-service
      -> content-service
      -> relation-service
      -> message-service
      -> job-service
```

说明：

- `gateway`
  - HTTP 入口
  - 参数校验
  - 统一响应
  - 调用内部 RPC
- `passport-service`
  - 平台注册
  - 平台登录
  - Access Token / Refresh Token
  - 平台账号状态
- `school-service`
  - 校园登录页会话初始化
  - 验证码获取
  - 校园账号密码加密存储
  - TGT / Cookie / Session 管理
  - 课表、成绩、个人信息查询
  - 学校侧 session 失效后自动重登
- `content-service`
  - 发帖、评论、点赞、收藏
- `relation-service`
  - 关注、互关好友、拉黑
- `message-service`
  - 私信
  - 互动消息
- `job-service`
  - 毕业状态同步
  - 半年未登录自动销户
  - 清理学校侧 session

## 注册流程

### 1. 先校园授权

用户在平台注册页点击“校园授权”后：

1. App 调 `school-service.PreAuthorize`
2. `school-service` 去学校登录页建立会话并拿验证码
3. 返回：
   - `auth_session_id`
   - `captcha_image_base64`

### 2. 用户输入校园账号密码

用户填写：

- `student_no`
- `school_password`
- `captcha_code`

然后调用 `school-service.Authorize`

`school-service` 内部：

- 用验证码、学号、校园密码登录学校系统
- 成功后生成 `school_auth_ticket`
- 暂存授权结果

返回：

- `school_auth_ticket`
- `student_no`
- `student_name`
- `student_status`

### 3. 再完成平台注册

用户继续填写：

- `nickname`
- `platform_password`

然后调用 `passport-service.Register`

入参带上：

- `student_no`
- `nickname`
- `platform_password`
- `school_auth_ticket`

`passport-service` 完成：

- 创建平台用户
- 调 `school-service.BindPlatformUser`
- 绑定 `user_id <-> student_no`

## 登录流程

平台注册完成后，用户后续登录只走平台账号体系：

1. 用户输入学号 + 平台密码
2. `passport-service` 签发平台 token
3. 发帖、评论、点赞等平台能力不依赖学校系统

## 学校信息查询流程

例如查课表、查成绩：

1. App 请求平台接口
2. `gateway` 调 `school-service.GetStudentSchedule/GetStudentGrade`
3. `school-service` 根据 `user_id` 找到绑定的学校账号
4. 从数据库或 Redis 读取学校侧 session
5. 如果 session 有效，直接请求学校
6. 如果 session 失效：
   - 用数据库中加密保存的校园密码自动重登
   - 重登成功则继续请求学校
   - 重登失败则返回 `REAUTH_REQUIRED`

## 数据存储建议

### passport-service

平台用户表：

- `user_id`
- `student_no`
- `nickname`
- `password_hash`
- `account_status`
- `permission_status`
- `last_login_at`
- `last_active_at`
- `graduation_status`
- `deleted_at`
- `created_at`
- `updated_at`

### school-service

学校账号表：

- `id`
- `user_id`
- `student_no`
- `encrypted_school_password`
- `bind_status`
- `last_authorized_at`
- `last_login_at`
- `created_at`
- `updated_at`

学校 session 表：

- `id`
- `user_id`
- `student_no`
- `tgt`
- `cookie_json`
- `session_status`
- `expires_at`
- `updated_at`

### Redis

- `school:auth:{auth_session_id}`
- `school:ticket:{school_auth_ticket}`
- `school:session:{user_id}`

## 权限规则

### 毕业后只读

不建议在每次发帖、评论、点赞时实时去学校查状态。

推荐做法：

- `job-service` 定时同步学生状态
- 同步到 `passport-service` 的 `permission_status`

状态建议：

- `NORMAL`
- `READ_ONLY`
- `BANNED`
- `DEACTIVATED`

规则：

- 在校：`NORMAL`
- 毕业：`READ_ONLY`
- 半年未登录：`DEACTIVATED`

## 半年未登录自动销户

建议先做软删除，不要直接物理删除：

1. 定时任务扫描 `last_login_at`
2. 超过 180 天
3. 改成 `DEACTIVATED`
4. 清理：
   - Refresh Token
   - 学校侧 session
   - 敏感缓存
5. 保留内容数据或做匿名化

## 安全建议

- 平台密码只在 `passport-service` 保存，使用不可逆 hash
- 校园密码只在 `school-service` 保存，使用可解密加密存储
- `passport-service` 不要接触校园密码
- `school-service` 不对外暴露 TGT/cookie/session 细节
- 查询学校数据统一传 `user_id`
