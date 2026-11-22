package handler

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/impb"
	"hfjj/backend/proto/passportpb"
)

const websocketGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

type realtimeIncomingMessage struct {
	Type             string `json:"type"`
	TargetUserID     int64  `json:"target_user_id"`
	PeerUserID       int64  `json:"peer_user_id"`
	Page             int64  `json:"page"`
	ClientMsgNo      string `json:"client_msg_no"`
	ContentText      string `json:"content_text"`
	ChannelID        string `json:"channel_id"`
	ConversationType int32  `json:"conversation_type"`
	MessageSeq       int64  `json:"message_seq"`
}

type realtimeWSClient struct {
	conn      net.Conn
	send      chan []byte
	done      chan struct{}
	closeOnce sync.Once
	writeMu   sync.Mutex
}

func newRealtimeWSClient(conn net.Conn) *realtimeWSClient {
	return &realtimeWSClient{
		conn: conn,
		send: make(chan []byte, 32),
		done: make(chan struct{}),
	}
}

func (c *realtimeWSClient) SendJSON(payload map[string]any) bool {
	raw, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	frame := encodeWSFrame(0x1, raw)
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- frame:
		return true
	case <-c.done:
		return false
	default:
		return false
	}
}

func (c *realtimeWSClient) sendControl(opcode byte, payload []byte) bool {
	frame := encodeWSFrame(opcode, payload)
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err := c.conn.Write(frame)
	return err == nil
}

func (c *realtimeWSClient) writeLoop() {
	for {
		select {
		case <-c.done:
			return
		case frame := <-c.send:
			c.writeMu.Lock()
			_, err := c.conn.Write(frame)
			c.writeMu.Unlock()
			if err != nil {
				c.Close()
				return
			}
		}
	}
}

func (c *realtimeWSClient) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

func imRealtimeWSHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyRealtimeUser(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		conn, reader, ok := acceptWebSocket(w, r)
		if !ok {
			return
		}

		client := newRealtimeWSClient(conn)
		svcCtx.Realtime.Register(user.GetUserId(), client)
		defer svcCtx.Realtime.Unregister(user.GetUserId(), client)
		defer client.Close()

		go client.writeLoop()
		client.SendJSON(map[string]any{
			"type":    "connected",
			"user_id": user.GetUserId(),
		})

		for {
			opcode, payload, err := readWSFrame(reader)
			if err != nil {
				return
			}
			switch opcode {
			case 0x1:
				handleRealtimeTextMessage(r.Context(), svcCtx, client, user, payload)
			case 0x8:
				_ = client.sendControl(0x8, nil)
				return
			case 0x9:
				_ = client.sendControl(0xA, payload)
			}
		}
	}
}

func handleRealtimeTextMessage(ctx context.Context, svcCtx *svc.ServiceContext, client *realtimeWSClient, user *passportpb.UserProfile, payload []byte) {
	var msg realtimeIncomingMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		client.SendJSON(map[string]any{"type": "error", "message": "\u0057\u0065\u0062Socket\u6570\u636e\u65e0\u6548"})
		return
	}

	switch strings.TrimSpace(msg.Type) {
	case "ping":
		client.SendJSON(map[string]any{"type": "pong", "ts": time.Now().UnixMilli()})
	case "send_direct_message":
		handleRealtimeSendDirectMessage(ctx, svcCtx, client, user, msg)
	case "mark_conversation_read":
		handleRealtimeMarkConversationRead(ctx, svcCtx, client, user, msg)
	case "sync_direct_conversations":
		handleRealtimeSyncDirectConversations(ctx, svcCtx, client, user, msg)
	case "sync_direct_messages":
		handleRealtimeSyncDirectMessages(ctx, svcCtx, client, user, msg)
	case "check_direct_message_permission":
		handleRealtimeCheckDirectMessagePermission(ctx, svcCtx, client, user, msg)
	default:
		client.SendJSON(map[string]any{"type": "error", "message": "\u672a\u77e5\u7684WebSocket\u6d88\u606f\u7c7b\u578b"})
	}
}

func handleRealtimeSyncDirectConversations(ctx context.Context, svcCtx *svc.ServiceContext, client *realtimeWSClient, user *passportpb.UserProfile, msg realtimeIncomingMessage) {
	page := msg.Page
	if page <= 0 {
		page = 1
	}

	resp, err := svcCtx.ImRPC.ListDirectConversations(ctx, &impb.ListDirectConversationsRequest{
		UserId: user.GetUserId(),
		Page:   page,
	})
	if err != nil {
		client.SendJSON(map[string]any{
			"type":    "sync_direct_conversations_result",
			"ok":      false,
			"page":    page,
			"message": err.Error(),
		})
		return
	}

	items := make([]map[string]any, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, directConversationToMap(item))
	}

	client.SendJSON(map[string]any{
		"type":    "sync_direct_conversations_result",
		"ok":      resp.GetBase().GetCode() == impb.IMCode_IM_CODE_OK,
		"page":    page,
		"code":    int32(resp.GetBase().GetCode()),
		"message": resp.GetBase().GetMessage(),
		"data": map[string]any{
			"items":    items,
			"has_more": resp.GetHasMore(),
		},
	})
}

func handleRealtimeSyncDirectMessages(ctx context.Context, svcCtx *svc.ServiceContext, client *realtimeWSClient, user *passportpb.UserProfile, msg realtimeIncomingMessage) {
	page := msg.Page
	if page <= 0 {
		page = 1
	}
	if msg.PeerUserID <= 0 {
		client.SendJSON(map[string]any{
			"type":         "sync_direct_messages_result",
			"ok":           false,
			"peer_user_id": msg.PeerUserID,
			"message":      "\u5bf9\u65b9\u7528\u6237ID\u65e0\u6548",
		})
		return
	}

	resp, err := svcCtx.ImRPC.ListDirectMessages(ctx, &impb.ListDirectMessagesRequest{
		UserId:     user.GetUserId(),
		PeerUserId: msg.PeerUserID,
		Page:       page,
	})
	if err != nil {
		client.SendJSON(map[string]any{
			"type":         "sync_direct_messages_result",
			"ok":           false,
			"peer_user_id": msg.PeerUserID,
			"page":         page,
			"message":      err.Error(),
		})
		return
	}

	items := make([]map[string]any, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, directMessageToMap(item))
	}

	client.SendJSON(map[string]any{
		"type":         "sync_direct_messages_result",
		"ok":           resp.GetBase().GetCode() == impb.IMCode_IM_CODE_OK,
		"peer_user_id": msg.PeerUserID,
		"page":         page,
		"code":         int32(resp.GetBase().GetCode()),
		"message":      resp.GetBase().GetMessage(),
		"data": map[string]any{
			"items":    items,
			"has_more": resp.GetHasMore(),
		},
	})
}

func handleRealtimeCheckDirectMessagePermission(ctx context.Context, svcCtx *svc.ServiceContext, client *realtimeWSClient, user *passportpb.UserProfile, msg realtimeIncomingMessage) {
	if msg.TargetUserID <= 0 {
		client.SendJSON(map[string]any{
			"type":           "check_direct_message_permission_result",
			"ok":             false,
			"target_user_id": msg.TargetUserID,
			"message":        "\u76ee\u6807\u7528\u6237ID\u65e0\u6548",
		})
		return
	}

	resp, err := svcCtx.ImRPC.CheckDirectMessagePermission(ctx, &impb.CheckDirectMessagePermissionRequest{
		SenderUserId:   user.GetUserId(),
		ReceiverUserId: msg.TargetUserID,
	})
	if err != nil {
		client.SendJSON(map[string]any{
			"type":           "check_direct_message_permission_result",
			"ok":             false,
			"target_user_id": msg.TargetUserID,
			"message":        err.Error(),
		})
		return
	}

	client.SendJSON(map[string]any{
		"type":           "check_direct_message_permission_result",
		"ok":             resp.GetBase().GetCode() == impb.IMCode_IM_CODE_OK,
		"target_user_id": msg.TargetUserID,
		"code":           int32(resp.GetBase().GetCode()),
		"message":        resp.GetBase().GetMessage(),
		"permission":     dmPermissionToMap(resp.GetPermission()),
	})
}

func handleRealtimeSendDirectMessage(ctx context.Context, svcCtx *svc.ServiceContext, client *realtimeWSClient, user *passportpb.UserProfile, msg realtimeIncomingMessage) {
	content := strings.TrimSpace(msg.ContentText)
	clientMsgNo := strings.TrimSpace(msg.ClientMsgNo)
	if msg.TargetUserID <= 0 || content == "" || clientMsgNo == "" {
		client.SendJSON(map[string]any{
			"type":          "send_direct_message_result",
			"client_msg_no": clientMsgNo,
			"ok":            false,
			"message":       "\u79c1\u4fe1\u8bf7\u6c42\u65e0\u6548",
		})
		return
	}

	resp, err := svcCtx.ImRPC.SendDirectMessage(ctx, &impb.SendDirectMessageRequest{
		SenderUserId:   user.GetUserId(),
		ReceiverUserId: msg.TargetUserID,
		ClientMsgNo:    clientMsgNo,
		ContentText:    content,
	})
	if err != nil {
		client.SendJSON(map[string]any{
			"type":          "send_direct_message_result",
			"client_msg_no": clientMsgNo,
			"ok":            false,
			"message":       err.Error(),
		})
		return
	}
	if resp.GetBase().GetCode() != impb.IMCode_IM_CODE_OK {
		client.SendJSON(map[string]any{
			"type":          "send_direct_message_result",
			"client_msg_no": clientMsgNo,
			"ok":            false,
			"code":          int32(resp.GetBase().GetCode()),
			"message":       resp.GetBase().GetMessage(),
			"permission":    dmPermissionToMap(resp.GetPermission()),
		})
		return
	}

	receipt := resp.GetReceipt()
	if receipt == nil {
		client.SendJSON(map[string]any{
			"type":          "send_direct_message_result",
			"client_msg_no": clientMsgNo,
			"ok":            false,
			"message":       "\u7f3a\u5c11\u79c1\u4fe1\u56de\u6267",
		})
		return
	}
	createdAt := time.Now().Format(time.RFC3339Nano)
	client.SendJSON(map[string]any{
		"type":          "send_direct_message_result",
		"client_msg_no": clientMsgNo,
		"ok":            true,
		"receipt": map[string]any{
			"message_id":        receipt.GetMessageId(),
			"message_seq":       receipt.GetMessageSeq(),
			"client_msg_no":     receipt.GetClientMsgNo(),
			"channel_id":        receipt.GetChannelId(),
			"conversation_type": int32(receipt.GetConversationType()),
		},
	})
	broadcastDirectMessageEvent(
		svcCtx,
		user.GetUserId(),
		msg.TargetUserID,
		clientMsgNo,
		content,
		receipt.GetMessageId(),
		receipt.GetMessageSeq(),
		"sent",
		createdAt,
	)
}

func handleRealtimeMarkConversationRead(ctx context.Context, svcCtx *svc.ServiceContext, client *realtimeWSClient, user *passportpb.UserProfile, msg realtimeIncomingMessage) {
	peerUserID := msg.PeerUserID
	channelID := strings.TrimSpace(msg.ChannelID)
	if peerUserID <= 0 && strings.HasPrefix(channelID, "u_") {
		peerUserID, _ = strconv.ParseInt(strings.TrimPrefix(channelID, "u_"), 10, 64)
	}
	if peerUserID <= 0 {
		client.SendJSON(map[string]any{
			"type":         "mark_conversation_read_result",
			"ok":           false,
			"peer_user_id": peerUserID,
			"message":      "\u5df2\u8bfb\u56de\u6267\u5bf9\u7aef\u65e0\u6548",
		})
		return
	}
	if channelID == "" {
		channelID = fmt.Sprintf("u_%d", peerUserID)
	}
	if msg.ConversationType == 0 {
		msg.ConversationType = int32(impb.ConversationType_CONVERSATION_TYPE_PERSONAL)
	}

	resp, err := svcCtx.ImRPC.MarkConversationRead(ctx, &impb.MarkConversationReadRequest{
		UserId:           user.GetUserId(),
		ChannelId:        channelID,
		ConversationType: impb.ConversationType(msg.ConversationType),
		MessageSeq:       msg.MessageSeq,
	})
	if err != nil {
		client.SendJSON(map[string]any{
			"type":         "mark_conversation_read_result",
			"ok":           false,
			"peer_user_id": peerUserID,
			"message_seq":  msg.MessageSeq,
			"message":      err.Error(),
		})
		return
	}
	if resp.GetBase().GetCode() != impb.IMCode_IM_CODE_OK {
		client.SendJSON(map[string]any{
			"type":         "mark_conversation_read_result",
			"ok":           false,
			"peer_user_id": peerUserID,
			"message_seq":  msg.MessageSeq,
			"code":         int32(resp.GetBase().GetCode()),
			"message":      resp.GetBase().GetMessage(),
		})
		return
	}

	client.SendJSON(map[string]any{
		"type":         "mark_conversation_read_result",
		"ok":           true,
		"peer_user_id": peerUserID,
		"message_seq":  msg.MessageSeq,
	})

	event := map[string]any{
		"type":           "conversation_read",
		"reader_user_id": user.GetUserId(),
		"peer_user_id":   peerUserID,
		"message_seq":    msg.MessageSeq,
	}
	svcCtx.Realtime.Broadcast(user.GetUserId(), event)
	svcCtx.Realtime.Broadcast(peerUserID, event)
}

func broadcastDirectMessageEvent(svcCtx *svc.ServiceContext, senderUserID, receiverUserID int64, clientMsgNo, contentText string, messageID, messageSeq int64, status, createdAt string) {
	if svcCtx == nil || svcCtx.Realtime == nil {
		return
	}
	if strings.TrimSpace(createdAt) == "" {
		createdAt = time.Now().Format(time.RFC3339Nano)
	}
	event := map[string]any{
		"type": "direct_message",
		"message": map[string]any{
			"sender_user_id":   senderUserID,
			"receiver_user_id": receiverUserID,
			"client_msg_no":    clientMsgNo,
			"content_text":     contentText,
			"message_id":       messageID,
			"message_seq":      messageSeq,
			"status":           status,
			"created_at":       createdAt,
		},
		"sender":   loadUserSummary(svcCtx, senderUserID),
		"receiver": loadUserSummary(svcCtx, receiverUserID),
	}
	svcCtx.Realtime.Broadcast(senderUserID, event)
	if receiverUserID != senderUserID {
		svcCtx.Realtime.Broadcast(receiverUserID, event)
	}
}

func verifyRealtimeUser(w http.ResponseWriter, r *http.Request, svcCtx *svc.ServiceContext) (*passportpb.UserProfile, bool) {
	token := strings.TrimSpace(r.URL.Query().Get("access_token"))
	if token == "" {
		authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		}
	}
	if token == "" {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{Code: int32(passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID), Message: "\u7f3a\u5c11\u8bbf\u95ee\u4ee4\u724c"})
		return nil, false
	}

	resp, err := svcCtx.PassportRPC.VerifyAccessToken(r.Context(), &passportpb.VerifyAccessTokenRequest{
		AccessToken: token,
	})
	if err != nil {
		writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
		return nil, false
	}
	if resp.GetBase().GetCode() != passportpb.PassportCode_PASSPORT_CODE_OK || !resp.GetValid() || resp.GetUser() == nil {
		writeJSON(w, http.StatusUnauthorized, jsonResponse{
			Code:    int32(resp.GetBase().GetCode()),
			Message: resp.GetBase().GetMessage(),
		})
		return nil, false
	}
	return resp.GetUser(), true
}

func acceptWebSocket(w http.ResponseWriter, r *http.Request) (net.Conn, *bufio.Reader, bool) {
	if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Upgrade")), "websocket") {
		http.Error(w, "\u7f3a\u5c11WebSocket\u5347\u7ea7\u8bf7\u6c42", http.StatusBadRequest)
		return nil, nil, false
	}
	key := strings.TrimSpace(r.Header.Get("Sec-WebSocket-Key"))
	if key == "" {
		http.Error(w, "\u7f3a\u5c11WebSocket\u63e1\u624b\u5bc6\u94a5", http.StatusBadRequest)
		return nil, nil, false
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "\u5f53\u524d\u8fde\u63a5\u4e0d\u652f\u6301WebSocket\u52ab\u6301", http.StatusInternalServerError)
		return nil, nil, false
	}
	conn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, false
	}

	accept := websocketAcceptKey(key)
	_, _ = fmt.Fprintf(rw, "HTTP/1.1 101 Switching Protocols\r\n")
	_, _ = fmt.Fprintf(rw, "Upgrade: websocket\r\n")
	_, _ = fmt.Fprintf(rw, "Connection: Upgrade\r\n")
	_, _ = fmt.Fprintf(rw, "Sec-WebSocket-Accept: %s\r\n", accept)
	_, _ = fmt.Fprintf(rw, "\r\n")
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, nil, false
	}
	return conn, rw.Reader, true
}

func websocketAcceptKey(key string) string {
	sum := sha1.Sum([]byte(key + websocketGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func readWSFrame(reader *bufio.Reader) (byte, []byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(reader, header); err != nil {
		return 0, nil, err
	}
	opcode := header[0] & 0x0F
	masked := header[1]&0x80 != 0
	length := uint64(header[1] & 0x7F)
	switch length {
	case 126:
		extended := make([]byte, 2)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return 0, nil, err
		}
		length = uint64(binary.BigEndian.Uint16(extended))
	case 127:
		extended := make([]byte, 8)
		if _, err := io.ReadFull(reader, extended); err != nil {
			return 0, nil, err
		}
		length = binary.BigEndian.Uint64(extended)
	}
	if length > 1<<20 {
		return 0, nil, errors.New("\u0057\u0065\u0062Socket\u8d1f\u8f7d\u8fc7\u5927")
	}

	var maskKey [4]byte
	if masked {
		if _, err := io.ReadFull(reader, maskKey[:]); err != nil {
			return 0, nil, err
		}
	}

	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(reader, payload); err != nil {
			return 0, nil, err
		}
	}
	if masked {
		for i := range payload {
			payload[i] ^= maskKey[i%4]
		}
	}
	return opcode, payload, nil
}

func encodeWSFrame(opcode byte, payload []byte) []byte {
	header := []byte{0x80 | (opcode & 0x0F)}
	length := len(payload)
	switch {
	case length < 126:
		header = append(header, byte(length))
	case length <= 0xFFFF:
		header = append(header, 126, byte(length>>8), byte(length))
	default:
		header = append(header, 127)
		var extended [8]byte
		binary.BigEndian.PutUint64(extended[:], uint64(length))
		header = append(header, extended[:]...)
	}
	return append(header, payload...)
}
