package logic

import (
	"context"
	"strings"
	"time"

	"hfjj/backend/proto/schoolpb"
	"hfjj/backend/school-service/internal/svc"
	"hfjj/backend/school-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/metadata"
)

type BindPlatformUserLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewBindPlatformUserLogic(ctx context.Context, svcCtx *svc.ServiceContext) *BindPlatformUserLogic {
	return &BindPlatformUserLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// BindPlatformUser 将一次临时校园授权结果正式绑定到平台 user_id。
func (l *BindPlatformUserLogic) BindPlatformUser(in *schoolpb.BindPlatformUserRequest) (*schoolpb.BindPlatformUserResponse, error) {
	if in.GetUserId() <= 0 || strings.TrimSpace(in.GetSchoolAuthTicket()) == "" {
		return &schoolpb.BindPlatformUserResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_AUTH_TICKET_INVALID, "user_id 和 school_auth_ticket 不能为空"),
		}, nil
	}

	existingByUser, existsByUser, err := l.svcCtx.SchoolAccountStore.GetByUserID(l.ctx, in.GetUserId())
	if err != nil {
		return &schoolpb.BindPlatformUserResponse{
			Base: statusFromError(err),
		}, nil
	}

	authorized, ok, err := l.svcCtx.AuthorizedStore.Get(l.ctx, in.GetSchoolAuthTicket())
	if err != nil {
		return &schoolpb.BindPlatformUserResponse{
			Base: statusFromError(err),
		}, nil
	}
	if !ok {
		if allowRegisterRecovery(l.ctx) && existsByUser && existingByUser.BindStatus == 1 && strings.TrimSpace(existingByUser.StudentNo) != "" {
			return &schoolpb.BindPlatformUserResponse{
				Base:      newStatus(schoolpb.SchoolCode_SCHOOL_CODE_OK, "ok"),
				UserId:    existingByUser.UserID,
				StudentNo: existingByUser.StudentNo,
			}, nil
		}
		return &schoolpb.BindPlatformUserResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_AUTH_TICKET_INVALID, "school_auth_ticket 无效或已过期"),
		}, nil
	}

	if existsByUser && strings.TrimSpace(existingByUser.StudentNo) != "" && existingByUser.StudentNo != authorized.StudentNo {
		return &schoolpb.BindPlatformUserResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_AUTH_TICKET_INVALID, "当前账号只能重新授权已绑定的学号"),
		}, nil
	}

	existingByStudentNo, existsByStudentNo, err := l.svcCtx.SchoolAccountStore.GetByStudentNo(l.ctx, authorized.StudentNo)
	if err != nil {
		return &schoolpb.BindPlatformUserResponse{
			Base: statusFromError(err),
		}, nil
	}
	if existsByStudentNo && existingByStudentNo.UserID != in.GetUserId() {
		return &schoolpb.BindPlatformUserResponse{
			Base: newStatus(schoolpb.SchoolCode_SCHOOL_CODE_ALREADY_BOUND, "该学号已经绑定到其他平台用户"),
		}, nil
	}

	account, err := l.svcCtx.SchoolAccountStore.Save(l.ctx, types.SchoolAccount{
		UserID:        in.GetUserId(),
		StudentNo:     authorized.StudentNo,
		StudentName:   strings.TrimSpace(authorized.StudentName),
		Password:      "",
		StudentStatus: reconcileStudentStatus(authorized.StudentStatus, authorized.AdmissionAt, authorized.GraduationAt, time.Now()),
		BindStatus:    1,
		AdmissionAt:   authorized.AdmissionAt,
		GraduationAt:  authorized.GraduationAt,
		UpdatedAt:     time.Now(),
	})
	if err != nil {
		return &schoolpb.BindPlatformUserResponse{
			Base: statusFromError(err),
		}, nil
	}

	if err := l.svcCtx.AuthorizedStore.Delete(l.ctx, in.GetSchoolAuthTicket()); err != nil {
		return &schoolpb.BindPlatformUserResponse{
			Base: statusFromError(err),
		}, nil
	}

	return &schoolpb.BindPlatformUserResponse{
		Base:      newStatus(schoolpb.SchoolCode_SCHOOL_CODE_OK, "ok"),
		UserId:    account.UserID,
		StudentNo: account.StudentNo,
	}, nil
}

func allowRegisterRecovery(ctx context.Context) bool {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return false
	}

	values := md.Get("x-hfjj-register-recovery")
	if len(values) == 0 {
		return false
	}

	return strings.EqualFold(strings.TrimSpace(values[0]), "1")
}
