package server

import (
	"context"
	"strings"

	"hfjj/backend/proto/updatepb"
	"hfjj/backend/update-service/internal/appversion"
	"hfjj/backend/update-service/internal/svc"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UpdateServiceServer 负责对外暴露版本 RPC 能力。
type UpdateServiceServer struct {
	ctx *svc.ServiceContext
	updatepb.UnimplementedUpdateServiceServer
}

func NewUpdateServiceServer(ctx *svc.ServiceContext) *UpdateServiceServer {
	return &UpdateServiceServer{ctx: ctx}
}

// GetLatestVersion 返回指定平台当前激活的最新版本。
func (s *UpdateServiceServer) GetLatestVersion(ctx context.Context, in *updatepb.GetLatestVersionRequest) (*updatepb.GetLatestVersionResponse, error) {
	record, ok, err := s.ctx.AppVersion.Latest(ctx, in.GetPlatform())
	if err != nil {
		return nil, err
	}
	if !ok {
		return &updatepb.GetLatestVersionResponse{}, nil
	}
	return &updatepb.GetLatestVersionResponse{
		Version: toProtoVersion(record),
	}, nil
}

// ListAppVersions 返回后台使用的版本列表。
func (s *UpdateServiceServer) ListAppVersions(ctx context.Context, in *updatepb.ListAppVersionsRequest) (*updatepb.ListAppVersionsResponse, error) {
	items, err := s.ctx.AppVersion.List(ctx, in.GetPlatform(), int(in.GetLimit()))
	if err != nil {
		return nil, err
	}

	resp := &updatepb.ListAppVersionsResponse{
		Items: make([]*updatepb.AppVersion, 0, len(items)),
	}
	for _, item := range items {
		resp.Items = append(resp.Items, toProtoVersion(item))
	}
	return resp, nil
}

// UpsertAppVersion 创建或更新一条版本记录。
func (s *UpdateServiceServer) UpsertAppVersion(ctx context.Context, in *updatepb.UpsertAppVersionRequest) (*updatepb.UpsertAppVersionResponse, error) {
	if strings.TrimSpace(in.GetVersionName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "version_name is required")
	}

	record, err := s.ctx.AppVersion.Upsert(ctx, appversion.UpsertInput{
		VersionName:         in.GetVersionName(),
		VersionCode:         in.GetVersionCode(),
		Platform:            in.GetPlatform(),
		DownloadURL:         in.GetDownloadUrl(),
		AppStoreURL:         in.GetAppStoreUrl(),
		ForceUpdate:         in.GetForceUpdate(),
		MinSupportedVersion: in.GetMinSupportedVersion(),
		UpdateLog:           in.GetUpdateLog(),
		IsActive:            in.GetIsActive(),
		CreatedBy:           in.GetCreatedBy(),
	})
	if err != nil {
		return nil, err
	}

	return &updatepb.UpsertAppVersionResponse{
		Version: toProtoVersion(record),
	}, nil
}

func toProtoVersion(record appversion.Record) *updatepb.AppVersion {
	return &updatepb.AppVersion{
		Id:                  record.ID,
		VersionName:         record.VersionName,
		VersionCode:         record.VersionCode,
		Platform:            record.Platform,
		DownloadUrl:         record.DownloadURL,
		AppStoreUrl:         record.AppStoreURL,
		ForceUpdate:         record.ForceUpdate,
		MinSupportedVersion: record.MinSupportedVersion,
		UpdateLog:           record.UpdateLog,
		IsActive:            record.IsActive,
		CreatedAt:           record.CreatedAt.Format("2006-01-02 15:04:05"),
	}
}
