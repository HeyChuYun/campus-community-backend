package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/svc"
	"hfjj/backend/proto/captchapb"
	"hfjj/backend/proto/passportpb"
	"hfjj/backend/proto/schoolpb"
)

type rebindSchoolRequest struct {
	SchoolAuthTicket string `json:"school_auth_ticket"`
}

type refreshSchoolSessionRequest struct {
	StudentNo      string `json:"student_no"`
	SchoolPassword string `json:"school_password"`
}

type changePasswordRequest struct {
	SchoolAuthTicket string `json:"school_auth_ticket"`
	NewPassword      string `json:"new_password"`
}

type resetPasswordBySchoolRequest struct {
	StudentNo        string `json:"student_no"`
	SchoolAuthTicket string `json:"school_auth_ticket"`
	NewPassword      string `json:"new_password"`
}

type resolvePasswordResetAccountRequest struct {
	Account string `json:"account"`
}

type gatewaySchoolAccount struct {
	UserID      int64
	StudentNo   string
	StudentName string
}

type gatewayLoginRecord struct {
	LoginIP string
	Device  string
	LoginAt time.Time
}

func accountSecurityHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		schoolAccount, _, err := queryGatewaySchoolAccountByUserID(svcCtx, user.GetUserId())
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		loginRecords, err := queryGatewayLoginRecordsByUserID(svcCtx, user.GetUserId(), 20)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		items := make([]map[string]any, 0, len(loginRecords))
		for _, item := range loginRecords {
			items = append(items, map[string]any{
				"login_ip": item.LoginIP,
				"device":   item.Device,
				"login_at": item.LoginAt.Format(time.RFC3339),
			})
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"account":       user.GetAccount(),
				"nickname":      user.GetNickname(),
				"student_no":    schoolAccount.StudentNo,
				"student_name":  schoolAccount.StudentName,
				"login_records": items,
			},
		})
	}
}

func rebindSchoolHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req rebindSchoolRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, _, ok := authorizeBearerToken(w, r, svcCtx, "profile.update", 0)
		if !ok || user == nil {
			return
		}

		resp, err := svcCtx.SchoolRPC.BindPlatformUser(r.Context(), &schoolpb.BindPlatformUserRequest{
			UserId:           user.GetUserId(),
			SchoolAuthTicket: strings.TrimSpace(req.SchoolAuthTicket),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeSchoolStatus(w, resp.GetBase(), map[string]any{
			"user_id":    resp.GetUserId(),
			"student_no": resp.GetStudentNo(),
		})
	}
}

func refreshSchoolSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req refreshSchoolSessionRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		account, exists, err := queryGatewaySchoolAccountByUserID(svcCtx, user.GetUserId())
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists || strings.TrimSpace(account.StudentNo) == "" {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(schoolpb.SchoolCode_SCHOOL_CODE_NOT_BOUND),
				Message: "current account is not bound to a campus student number",
			})
			return
		}

		studentNo := strings.TrimSpace(req.StudentNo)
		schoolPassword := strings.TrimSpace(req.SchoolPassword)
		if studentNo == "" || schoolPassword == "" {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "student_no and school_password are required",
			})
			return
		}
		if account.StudentNo != studentNo {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "only the currently bound campus student number can be refreshed",
			})
			return
		}

		preAuthorizeResp, err := svcCtx.SchoolRPC.PreAuthorize(r.Context(), &schoolpb.PreAuthorizeRequest{})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if preAuthorizeResp.GetBase().GetCode() != schoolpb.SchoolCode_SCHOOL_CODE_OK {
			writeSchoolStatus(w, preAuthorizeResp.GetBase(), nil)
			return
		}

		solveResp, err := svcCtx.CaptchaRPC.SolveCaptcha(r.Context(), &captchapb.SolveCaptchaRequest{
			ImageBase64: preAuthorizeResp.GetCaptchaImageBase64(),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(schoolpb.SchoolCode_SCHOOL_CODE_REAUTH_REQUIRED),
				Message: err.Error(),
			})
			return
		}
		if solveResp.GetBase().GetCode() != captchapb.CaptchaCode_CAPTCHA_CODE_OK || strings.TrimSpace(solveResp.GetCode()) == "" {
			message := strings.TrimSpace(solveResp.GetBase().GetMessage())
			if message == "" {
				message = "captcha solve failed"
			}
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(schoolpb.SchoolCode_SCHOOL_CODE_REAUTH_REQUIRED),
				Message: message,
			})
			return
		}

		authorizeResp, err := svcCtx.SchoolRPC.Authorize(r.Context(), &schoolpb.AuthorizeRequest{
			AuthSessionId:  preAuthorizeResp.GetAuthSessionId(),
			StudentNo:      studentNo,
			SchoolPassword: schoolPassword,
			CaptchaCode:    strings.TrimSpace(solveResp.GetCode()),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}

		writeSchoolStatus(w, authorizeResp.GetBase(), map[string]any{
			"student_no":   authorizeResp.GetStudentNo(),
			"student_name": authorizeResp.GetStudentName(),
			"campus_tgt":   authorizeResp.GetCampusTgt(),
		})
	}
}

func changeMyPasswordHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req changePasswordRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		user, token, ok := authorizeBearerToken(w, r, svcCtx, "profile.update", 0)
		if !ok || user == nil {
			return
		}

		schoolAccount, exists, err := queryGatewaySchoolAccountByUserID(svcCtx, user.GetUserId())
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists || strings.TrimSpace(schoolAccount.StudentNo) == "" {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(schoolpb.SchoolCode_SCHOOL_CODE_NOT_BOUND),
				Message: "current account is not bound to a campus student number",
			})
			return
		}

		bindResp, err := svcCtx.SchoolRPC.BindPlatformUser(r.Context(), &schoolpb.BindPlatformUserRequest{
			UserId:           user.GetUserId(),
			SchoolAuthTicket: strings.TrimSpace(req.SchoolAuthTicket),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if bindResp.GetBase().GetCode() != schoolpb.SchoolCode_SCHOOL_CODE_OK {
			writeSchoolStatus(w, bindResp.GetBase(), nil)
			return
		}

		resp, err := svcCtx.PassportRPC.ChangeMyPassword(r.Context(), &passportpb.ChangeMyPasswordRequest{
			AccessToken: token,
			NewPassword: req.NewPassword,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writePassportStatus(w, resp.GetBase(), nil)
	}
}

func resetPasswordBySchoolHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req resetPasswordBySchoolRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		studentNo := strings.TrimSpace(req.StudentNo)
		if studentNo == "" || strings.TrimSpace(req.SchoolAuthTicket) == "" || req.NewPassword == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "student_no, school_auth_ticket and new_password are required",
			})
			return
		}

		schoolAccount, exists, err := queryGatewaySchoolAccountByStudentNo(svcCtx, studentNo)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists || schoolAccount.UserID <= 0 {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(schoolpb.SchoolCode_SCHOOL_CODE_NOT_BOUND),
				Message: "current campus student number is not bound to a platform account",
			})
			return
		}

		bindResp, err := svcCtx.SchoolRPC.BindPlatformUser(r.Context(), &schoolpb.BindPlatformUserRequest{
			UserId:           schoolAccount.UserID,
			SchoolAuthTicket: strings.TrimSpace(req.SchoolAuthTicket),
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if bindResp.GetBase().GetCode() != schoolpb.SchoolCode_SCHOOL_CODE_OK {
			writeSchoolStatus(w, bindResp.GetBase(), nil)
			return
		}

		resp, err := svcCtx.PassportRPC.ResetPasswordByUserID(r.Context(), &passportpb.ResetPasswordByUserIDRequest{
			UserId:      schoolAccount.UserID,
			NewPassword: req.NewPassword,
		})
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		writePassportStatus(w, resp.GetBase(), nil)
	}
}

func resolvePasswordResetAccountHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req resolvePasswordResetAccountRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		rawAccount := req.Account
		account := strings.TrimSpace(rawAccount)
		if account == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "账号或学号不能为空",
			})
			return
		}
		if strings.ContainsAny(rawAccount, " \t\r\n") {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "账号或学号不能包含空格",
			})
			return
		}

		var (
			schoolAccount gatewaySchoolAccount
			exists        bool
			err           error
		)
		if isAllDigits(account) {
			schoolAccount, exists, err = queryGatewaySchoolAccountByStudentNo(svcCtx, account)
		} else {
			schoolAccount, exists, err = queryGatewaySchoolAccountByAccount(svcCtx, account)
		}
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists || schoolAccount.UserID <= 0 || strings.TrimSpace(schoolAccount.StudentNo) == "" {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    int32(schoolpb.SchoolCode_SCHOOL_CODE_NOT_BOUND),
				Message: "当前账号尚未绑定校园学号",
			})
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"student_no":   schoolAccount.StudentNo,
				"student_name": schoolAccount.StudentName,
			},
		})
	}
}

func queryGatewaySchoolAccountByUserID(svcCtx *svc.ServiceContext, userID int64) (gatewaySchoolAccount, bool, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil || userID <= 0 {
		return gatewaySchoolAccount{}, false, nil
	}
	account, ok, err := svcCtx.UserQuery.SchoolAccountByUserID(context.Background(), userID)
	return gatewaySchoolAccount{UserID: account.UserID, StudentNo: account.StudentNo, StudentName: account.StudentName}, ok, err
}

func queryGatewaySchoolAccountByStudentNo(svcCtx *svc.ServiceContext, studentNo string) (gatewaySchoolAccount, bool, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil || strings.TrimSpace(studentNo) == "" {
		return gatewaySchoolAccount{}, false, nil
	}
	account, ok, err := svcCtx.UserQuery.SchoolAccountByStudentNo(context.Background(), strings.TrimSpace(studentNo))
	return gatewaySchoolAccount{UserID: account.UserID, StudentNo: account.StudentNo, StudentName: account.StudentName}, ok, err
}

func queryGatewaySchoolAccountByAccount(svcCtx *svc.ServiceContext, account string) (gatewaySchoolAccount, bool, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil || strings.TrimSpace(account) == "" {
		return gatewaySchoolAccount{}, false, nil
	}
	user, ok, err := svcCtx.UserQuery.ByAccount(context.Background(), strings.TrimSpace(account))
	if err != nil || !ok || user.ID <= 0 {
		return gatewaySchoolAccount{}, false, err
	}
	return queryGatewaySchoolAccountByUserID(svcCtx, user.ID)
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func queryGatewayLoginRecordsByUserID(svcCtx *svc.ServiceContext, userID int64, limit int64) ([]gatewayLoginRecord, error) {
	if svcCtx == nil || svcCtx.UserQuery == nil || userID <= 0 {
		return []gatewayLoginRecord{}, nil
	}
	items, err := svcCtx.UserQuery.LoginRecordsByUserID(context.Background(), userID, limit)
	if err != nil {
		return nil, err
	}
	result := make([]gatewayLoginRecord, 0, len(items))
	for _, item := range items {
		result = append(result, gatewayLoginRecord{LoginIP: item.LoginIP, Device: item.Device, LoginAt: item.LoginAt})
	}
	return result, nil
}
