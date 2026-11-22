package captchasolverservice

import (
	"context"

	"hfjj/backend/proto/captchapb"

	"github.com/zeromicro/go-zero/zrpc"
	"google.golang.org/grpc"
)

type (
	BaseResp             = captchapb.BaseResp
	SolveCaptchaRequest  = captchapb.SolveCaptchaRequest
	SolveCaptchaResponse = captchapb.SolveCaptchaResponse

	CaptchaSolverService interface {
		SolveCaptcha(ctx context.Context, in *SolveCaptchaRequest, opts ...grpc.CallOption) (*SolveCaptchaResponse, error)
	}

	defaultCaptchaSolverService struct {
		cli zrpc.Client
	}
)

func NewCaptchaSolverService(cli zrpc.Client) CaptchaSolverService {
	return &defaultCaptchaSolverService{cli: cli}
}

func (m *defaultCaptchaSolverService) SolveCaptcha(ctx context.Context, in *SolveCaptchaRequest, opts ...grpc.CallOption) (*SolveCaptchaResponse, error) {
	client := captchapb.NewCaptchaSolverServiceClient(m.cli.Conn())
	return client.SolveCaptcha(ctx, in, opts...)
}
