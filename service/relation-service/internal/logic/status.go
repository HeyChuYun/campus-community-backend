package logic

import "hfjj/backend/proto/relationpb"

func newStatus(code relationpb.RelationCode, message string) *relationpb.BaseResp {
	return &relationpb.BaseResp{
		Code:    code,
		Message: message,
	}
}
