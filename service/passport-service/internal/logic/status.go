package logic

import "hfjj/backend/proto/passportpb"

// newStatus 用于统一构造 passport-service 的响应状态对象。
func newStatus(code passportpb.PassportCode, message string) *passportpb.BaseResp {
	return &passportpb.BaseResp{
		Code:    code,
		Message: message,
	}
}
