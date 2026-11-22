package logic

import (
	"context"
	"strings"

	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"
)

type UpdateMyProfileLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateMyProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateMyProfileLogic {
	return &UpdateMyProfileLogic{ctx: ctx, svcCtx: svcCtx}
}

// UpdateMyProfile 更新当前登录用户的展示资料。
func (l *UpdateMyProfileLogic) UpdateMyProfile(in *passportpb.UpdateMyProfileRequest) (*passportpb.UpdateMyProfileResponse, error) {
	// 第一步：先校验 access_token，确保只能修改自己的资料。
	operator, code, message, ok, err := loadUserByAccessToken(l.svcCtx, strings.TrimSpace(in.GetAccessToken()))
	if err != nil {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(code, err.Error()),
		}, nil
	}
	if !ok {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(code, message),
		}, nil
	}

	// 第二步：读取当前用户原始记录，后续在这个对象上做局部更新。
	user, found, err := l.svcCtx.UserStore.GetByID(operator.ID)
	if err != nil {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if !found {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_ACCESS_TOKEN_INVALID, "user not found"),
		}, nil
	}

	// 第三步：显示名称不能为空，且如果发生变更，需要做唯一性校验。
	rawNickname := in.GetNickname()
	nickname := normalizeNickname(rawNickname)
	if nickname == "" {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INVALID_ARGUMENT, "昵称不能为空"),
		}, nil
	}
	if code, message, ok := validatePlatformNickname(rawNickname); !ok {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(code, message),
		}, nil
	}
	if nickname != user.Nickname {
		exists, err := l.svcCtx.UserStore.ExistsNickname(nickname)
		if err != nil {
			return &passportpb.UpdateMyProfileResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
			}, nil
		}
		if exists {
			return &passportpb.UpdateMyProfileResponse{
				Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_NICKNAME_ALREADY_EXISTS, "nickname already exists"),
			}, nil
		}
	}

	// 第四步：写入昵称、简介、头像和个人主页展示图。
	// 当前允许前端先传本地测试路径、data:image 或远程 URL，后续接对象存储时再统一切换。
	user.Nickname = nickname
	user.Bio = strings.TrimSpace(in.GetBio())
	user.AvatarURL = strings.TrimSpace(in.GetAvatarUrl())
	user.CoverImageURL = strings.TrimSpace(in.GetCoverImageUrl())

	// 第五步：持久化用户资料，并清理授权快照缓存，确保 /me 和鉴权侧读取到最新资料。
	if err := l.svcCtx.UserStore.Save(user); err != nil {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}
	if err := l.svcCtx.AuthzStore.Delete(user.ID); err != nil {
		return &passportpb.UpdateMyProfileResponse{
			Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_INTERNAL_ERROR, err.Error()),
		}, nil
	}

	return &passportpb.UpdateMyProfileResponse{
		Base: newStatus(passportpb.PassportCode_PASSPORT_CODE_OK, "ok"),
		User: buildUserProfile(user),
	}, nil
}
