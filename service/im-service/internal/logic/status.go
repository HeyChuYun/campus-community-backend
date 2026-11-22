package logic

import "hfjj/backend/proto/impb"

func newStatus(code impb.IMCode, message string) *impb.BaseResp {
	return &impb.BaseResp{
		Code:    code,
		Message: message,
	}
}
