package server

import (
	"context"

	"hfjj/backend/captcha-solver-service/internal/solver"
	"hfjj/backend/proto/captchapb"
)

type CaptchaSolverServer struct {
	captchapb.UnimplementedCaptchaSolverServiceServer
}

func NewCaptchaSolverServer() *CaptchaSolverServer {
	return &CaptchaSolverServer{}
}

func (s *CaptchaSolverServer) SolveCaptcha(_ context.Context, in *captchapb.SolveCaptchaRequest) (*captchapb.SolveCaptchaResponse, error) {
	imageBase64 := in.GetImageBase64()
	if imageBase64 == "" {
		return &captchapb.SolveCaptchaResponse{
			Base: &captchapb.BaseResp{
				Code:    captchapb.CaptchaCode_CAPTCHA_CODE_INVALID_ARGUMENT,
				Message: "image_base64 is empty",
			},
		}, nil
	}

	result, err := solver.SolveBase64(imageBase64)
	if err != nil {
		return &captchapb.SolveCaptchaResponse{
			Base: &captchapb.BaseResp{
				Code:    captchapb.CaptchaCode_CAPTCHA_CODE_SOLVE_FAILED,
				Message: err.Error(),
			},
		}, nil
	}

	return &captchapb.SolveCaptchaResponse{
		Base: &captchapb.BaseResp{
			Code:    captchapb.CaptchaCode_CAPTCHA_CODE_OK,
			Message: "ok",
		},
		Code:       result.Code,
		Expression: result.Expression,
	}, nil
}
