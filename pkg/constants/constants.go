package constants

// 帖子状态
const (
	PostStatusNormal  int32 = 1
	PostStatusDeleted int32 = 3
	PostStatusHidden  int32 = 4
)

// 帖子可见性
const (
	PostVisibilityPublic        int32 = 1
	PostVisibilityFollowersOnly int32 = 2
	PostVisibilityPrivate       int32 = 3
)

// 内容类型
const (
	ContentTypeText  int32 = 1
	ContentTypeImage int32 = 2
	ContentTypeVideo int32 = 3
	ContentTypeGIF   int32 = 4
)

// 评论状态
const (
	CommentStatusNormal  int32 = 1
	CommentStatusDeleted int32 = 3
)

// 账号状态
const (
	AccountStatusNormal      int32 = 1
	AccountStatusReadOnly    int32 = 2
	AccountStatusBlocked     int32 = 3
	AccountStatusDeactivated int32 = 4
)

// 分页默认值
const (
	DefaultPageSize    int64 = 20
	MaxPageSize        int64 = 100
	DefaultPage        int64 = 1
	DefaultHotTopicNum int64 = 10
)

// Token前缀
const (
	TokenPrefixSession = "sid_"
	TokenPrefixAccess  = "ajti_"
	TokenPrefixRefresh = "rjti_"
	TokenPrefixToken   = "rtk_"
)

// 缓存TTL
const (
	CacheTTLHotTopics = 5 * 60  // 5分钟
	CacheTTLUserInfo  = 10 * 60 // 10分钟
	CacheTTLPostInfo  = 1 * 60  // 1分钟
)

// 限流配置
const (
	GlobalRateLimit = 100 // 全局每秒请求数
	GlobalBurst     = 200 // 全局突发请求数
	IPRateLimit     = 20  // IP每秒请求数
	IPBurst         = 40  // IP突发请求数
	UserRateLimit   = 50  // 用户每秒请求数
	UserBurst       = 100 // 用户突发请求数
)

// 错误码
const (
	CodeOK                  int32 = 0
	CodeInvalidArgument     int32 = 400
	CodeUnauthorized        int32 = 401
	CodePermissionDenied    int32 = 403
	CodeNotFound            int32 = 404
	CodeTooManyRequests     int32 = 429
	CodeInternalError       int32 = 500
	CodeRPCError            int32 = 9001
	CodeUpstreamInvalidResp int32 = 9003
)
