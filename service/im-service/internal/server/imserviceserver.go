package server

import (
	"context"

	"hfjj/backend/im-service/internal/logic"
	"hfjj/backend/im-service/internal/svc"
	"hfjj/backend/proto/impb"
)

type IMServiceServer struct {
	ctx *svc.ServiceContext
	impb.UnimplementedIMServiceServer
}

func NewIMServiceServer(ctx *svc.ServiceContext) *IMServiceServer {
	return &IMServiceServer{ctx: ctx}
}

// IssueConnectToken 负责为客户端签发 WuKongIM 连接凭证。
func (s *IMServiceServer) IssueConnectToken(ctx context.Context, in *impb.IssueConnectTokenRequest) (*impb.IssueConnectTokenResponse, error) {
	l := logic.NewIssueConnectTokenLogic(ctx, s.ctx)
	return l.IssueConnectToken(in)
}

// ListDirectConversations 负责查询当前用户的私信会话列表。
func (s *IMServiceServer) ListDirectConversations(ctx context.Context, in *impb.ListDirectConversationsRequest) (*impb.ListDirectConversationsResponse, error) {
	l := logic.NewListDirectConversationsLogic(ctx, s.ctx)
	return l.ListDirectConversations(in)
}

// ListDirectMessages 负责查询当前用户和指定对方之间的私信历史。
func (s *IMServiceServer) ListDirectMessages(ctx context.Context, in *impb.ListDirectMessagesRequest) (*impb.ListDirectMessagesResponse, error) {
	l := logic.NewListDirectMessagesLogic(ctx, s.ctx)
	return l.ListDirectMessages(in)
}

// CheckDirectMessagePermission 负责检查发送私信前的业务规则。
func (s *IMServiceServer) CheckDirectMessagePermission(ctx context.Context, in *impb.CheckDirectMessagePermissionRequest) (*impb.CheckDirectMessagePermissionResponse, error) {
	l := logic.NewCheckDirectMessagePermissionLogic(ctx, s.ctx)
	return l.CheckDirectMessagePermission(in)
}

// SendDirectMessage 负责真正发送一条私信消息。
func (s *IMServiceServer) SendDirectMessage(ctx context.Context, in *impb.SendDirectMessageRequest) (*impb.SendDirectMessageResponse, error) {
	l := logic.NewSendDirectMessageLogic(ctx, s.ctx)
	return l.SendDirectMessage(in)
}

// MarkConversationRead 负责同步某个会话的已读位置。
func (s *IMServiceServer) MarkConversationRead(ctx context.Context, in *impb.MarkConversationReadRequest) (*impb.MarkConversationReadResponse, error) {
	l := logic.NewMarkConversationReadLogic(ctx, s.ctx)
	return l.MarkConversationRead(in)
}

// ListInteractionMessages 负责查询互动消息列表。
func (s *IMServiceServer) ListInteractionMessages(ctx context.Context, in *impb.ListInteractionMessagesRequest) (*impb.ListInteractionMessagesResponse, error) {
	l := logic.NewListInteractionMessagesLogic(ctx, s.ctx)
	return l.ListInteractionMessages(in)
}

// MarkInteractionMessagesRead 负责标记互动消息已读。
func (s *IMServiceServer) MarkInteractionMessagesRead(ctx context.Context, in *impb.MarkInteractionMessagesReadRequest) (*impb.MarkInteractionMessagesReadResponse, error) {
	l := logic.NewMarkInteractionMessagesReadLogic(ctx, s.ctx)
	return l.MarkInteractionMessagesRead(in)
}

// ListSystemNotifications 负责查询系统通知列表。
func (s *IMServiceServer) ListSystemNotifications(ctx context.Context, in *impb.ListSystemNotificationsRequest) (*impb.ListSystemNotificationsResponse, error) {
	l := logic.NewListSystemNotificationsLogic(ctx, s.ctx)
	return l.ListSystemNotifications(in)
}

// MarkSystemNotificationsRead 负责标记系统通知已读。
func (s *IMServiceServer) MarkSystemNotificationsRead(ctx context.Context, in *impb.MarkSystemNotificationsReadRequest) (*impb.MarkSystemNotificationsReadResponse, error) {
	l := logic.NewMarkSystemNotificationsReadLogic(ctx, s.ctx)
	return l.MarkSystemNotificationsRead(in)
}

// CreateSystemNotification 负责创建系统通知。
func (s *IMServiceServer) CreateSystemNotification(ctx context.Context, in *impb.CreateSystemNotificationRequest) (*impb.CreateSystemNotificationResponse, error) {
	l := logic.NewCreateSystemNotificationLogic(ctx, s.ctx)
	return l.CreateSystemNotification(in)
}

// GetTotalUnreadCount 负责获取用户的总未读消息数。
func (s *IMServiceServer) GetTotalUnreadCount(ctx context.Context, in *impb.GetTotalUnreadCountRequest) (*impb.GetTotalUnreadCountResponse, error) {
	l := logic.NewGetTotalUnreadCountLogic(ctx, s.ctx)
	return l.GetTotalUnreadCount(in)
}
