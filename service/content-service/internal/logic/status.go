package logic

import "hfjj/backend/proto/contentpb"

func newStatus(code contentpb.ContentCode, message string) *contentpb.BaseResp {
	return &contentpb.BaseResp{
		Code:    code,
		Message: message,
	}
}
