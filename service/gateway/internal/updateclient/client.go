package updateclient

import (
	"context"
	"strings"

	"hfjj/backend/proto/updatepb"
)

type Client struct {
	rpc updatepb.UpdateServiceClient
}

type Version struct {
	ID                  int64  `json:"id"`
	LatestVersion       string `json:"latest_version"`
	VersionName         string `json:"version_name"`
	VersionCode         int64  `json:"version_code"`
	Platform            string `json:"platform"`
	DownloadURL         string `json:"download_url"`
	AppStoreURL         string `json:"app_store_url"`
	ForceUpdate         bool   `json:"force_update"`
	MinSupportedVersion string `json:"min_supported_version"`
	UpdateLog           string `json:"update_log"`
	IsActive            bool   `json:"is_active"`
	CreatedAt           string `json:"created_at"`
}

type UpsertInput struct {
	VersionName         string `json:"version_name"`
	Version             string `json:"version"`
	VersionCode         int64  `json:"version_code"`
	Platform            string `json:"platform"`
	DownloadURL         string `json:"download_url"`
	AppStoreURL         string `json:"app_store_url"`
	ForceUpdate         bool   `json:"force_update"`
	MinSupportedVersion string `json:"min_supported_version"`
	UpdateLog           string `json:"update_log"`
	IsActive            *bool  `json:"is_active"`
}

// New 创建更新服务 RPC 客户端包装器。
func New(rpc updatepb.UpdateServiceClient) *Client {
	if rpc == nil {
		panic("gateway update rpc client is nil")
	}
	return &Client{rpc: rpc}
}

func (c *Client) Latest(ctx context.Context, currentVersion string, platform string) (Version, error) {
	resp, err := c.rpc.GetLatestVersion(ctx, &updatepb.GetLatestVersionRequest{
		CurrentVersion: strings.TrimSpace(currentVersion),
		Platform:       strings.TrimSpace(platform),
	})
	if err != nil {
		return Version{}, err
	}
	return fromProtoVersion(resp.GetVersion()), nil
}

func (c *Client) List(ctx context.Context, platform string) ([]Version, error) {
	resp, err := c.rpc.ListAppVersions(ctx, &updatepb.ListAppVersionsRequest{
		Platform: strings.TrimSpace(platform),
		Limit:    100,
	})
	if err != nil {
		return nil, err
	}

	items := make([]Version, 0, len(resp.GetItems()))
	for _, item := range resp.GetItems() {
		items = append(items, fromProtoVersion(item))
	}
	return items, nil
}

func (c *Client) Upsert(ctx context.Context, input UpsertInput) (Version, error) {
	versionName := strings.TrimSpace(input.VersionName)
	if versionName == "" {
		versionName = strings.TrimSpace(input.Version)
	}
	isActive := true
	if input.IsActive != nil {
		isActive = *input.IsActive
	}

	resp, err := c.rpc.UpsertAppVersion(ctx, &updatepb.UpsertAppVersionRequest{
		VersionName:         versionName,
		VersionCode:         input.VersionCode,
		Platform:            strings.TrimSpace(input.Platform),
		DownloadUrl:         strings.TrimSpace(input.DownloadURL),
		AppStoreUrl:         strings.TrimSpace(input.AppStoreURL),
		ForceUpdate:         input.ForceUpdate,
		MinSupportedVersion: strings.TrimSpace(input.MinSupportedVersion),
		UpdateLog:           strings.TrimSpace(input.UpdateLog),
		IsActive:            isActive,
	})
	if err != nil {
		return Version{}, err
	}
	return fromProtoVersion(resp.GetVersion()), nil
}

func fromProtoVersion(item *updatepb.AppVersion) Version {
	if item == nil {
		return Version{}
	}
	return Version{
		ID:                  item.GetId(),
		LatestVersion:       item.GetVersionName(),
		VersionName:         item.GetVersionName(),
		VersionCode:         item.GetVersionCode(),
		Platform:            item.GetPlatform(),
		DownloadURL:         item.GetDownloadUrl(),
		AppStoreURL:         item.GetAppStoreUrl(),
		ForceUpdate:         item.GetForceUpdate(),
		MinSupportedVersion: item.GetMinSupportedVersion(),
		UpdateLog:           item.GetUpdateLog(),
		IsActive:            item.GetIsActive(),
		CreatedAt:           item.GetCreatedAt(),
	}
}
