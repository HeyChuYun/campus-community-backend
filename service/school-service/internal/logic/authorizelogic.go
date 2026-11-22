package logic

import (
	"context"
	"strings"
	"time"

	"hfjj/backend/proto/schoolpb"
	"hfjj/backend/school-service/internal/svc"
	"hfjj/backend/school-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type AuthorizeLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAuthorizeLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuthorizeLogic {
	return &AuthorizeLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Authorize 使用预授权会话、学号、校园密码和验证码完成校园登录。
func (l *AuthorizeLogic) Authorize(in *schoolpb.AuthorizeRequest) (*schoolpb.AuthorizeResponse, error) {
	if strings.TrimSpace(in.GetAuthSessionId()) == "" ||
		strings.TrimSpace(in.GetStudentNo()) == "" ||
		strings.TrimSpace(in.GetSchoolPassword()) == "" ||
		strings.TrimSpace(in.GetCaptchaCode()) == "" {
		return &schoolpb.AuthorizeResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_INVALID_CREDENTIAL, "auth_session_id、student_no、school_password、captcha_code 不能为空"),
		}, nil
	}

	session, ok, err := l.svcCtx.AuthStore.Get(l.ctx, in.GetAuthSessionId())
	if err != nil {
		return &schoolpb.AuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}
	if !ok {
		return &schoolpb.AuthorizeResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_AUTH_TICKET_INVALID, "auth_session_id 无效或已过期"),
		}, nil
	}

	result, err := l.svcCtx.CampusClient.Authorize(
		l.ctx,
		session,
		in.GetStudentNo(),
		in.GetSchoolPassword(),
		in.GetCaptchaCode(),
	)
	if err != nil {
		return &schoolpb.AuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}

	profile, err := l.svcCtx.CampusClient.GetStudentProfile(l.ctx, types.SchoolSession{
		StudentNo: result.StudentNo,
		TGT:       result.TGT,
		Ticket:    result.Ticket,
	})
	if err != nil {
		return &schoolpb.AuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}

	studentName := extractStudentNameFromProfile(profile, result.StudentName)
	if studentName == "" {
		return &schoolpb.AuthorizeResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_REMOTE_ERROR, "已完成校园登录，但未能从个人信息中解析出姓名"),
		}, nil
	}
	result.StudentName = studentName

	admissionAt := extractAdmissionAtFromProfile(profile)
	graduationAt := extractGraduationAtFromProfile(profile, admissionAt)
	studentStatus := schoolpb.StudentStatus(
		reconcileStudentStatus(
			extractStudentStatusFromProfile(profile),
			admissionAt,
			graduationAt,
			time.Now(),
		),
	)

	schoolAuthTicket := "sat_" + time.Now().Format("20060102150405.000000000")
	if err := l.svcCtx.AuthorizedStore.Set(l.ctx, schoolAuthTicket, types.AuthorizedTicket{
		SchoolAuthTicket: schoolAuthTicket,
		StudentNo:        result.StudentNo,
		StudentName:      result.StudentName,
		StudentStatus:    int32(studentStatus),
		AdmissionAt:      admissionAt,
		GraduationAt:     graduationAt,
		TGT:              result.TGT,
		Ticket:           result.Ticket,
		CreatedAt:        time.Now(),
	}); err != nil {
		return &schoolpb.AuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}

	if err := l.svcCtx.AuthStore.Delete(l.ctx, in.GetAuthSessionId()); err != nil {
		return &schoolpb.AuthorizeResponse{
			Base: statusFromError(err),
		}, nil
	}

	return &schoolpb.AuthorizeResponse{
		Base:             newStatus(schoolpb.SchoolCode_SCHOOL_CODE_OK, "ok"),
		SchoolAuthTicket: schoolAuthTicket,
		StudentNo:        result.StudentNo,
		StudentName:      result.StudentName,
		StudentStatus:    studentStatus,
		CampusTgt:        result.TGT,
	}, nil
}
