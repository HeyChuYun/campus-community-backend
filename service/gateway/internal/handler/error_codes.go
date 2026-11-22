package handler

const (
	// gatewayCodeRPCError 表示网关调用下游 RPC 失败，例如服务未启动、网络不可达、超时等。
	gatewayCodeRPCError int32 = 9001
	// gatewayCodeInvalidArgument 表示请求参数、查询参数或 JSON 结构不合法。
	gatewayCodeInvalidArgument int32 = 9002
	// gatewayCodeUpstreamInvalidResponse 表示下游虽然返回成功，但关键响应字段缺失。
	gatewayCodeUpstreamInvalidResponse int32 = 9003
	// gatewayCodeNotFound 表示网关聚合时确认资源不存在。
	gatewayCodeNotFound int32 = 9004
)
