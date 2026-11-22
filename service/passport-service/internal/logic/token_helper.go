package logic

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"hfjj/backend/passport-service/internal/repository"
	"hfjj/backend/passport-service/internal/svc"
	"hfjj/backend/proto/passportpb"

	"github.com/golang-jwt/jwt/v4"
)

type accessClaims struct {
	UserID    int64  `json:"uid"`
	SessionID string `json:"sid"`
	Device    string `json:"device,omitempty"`
	jwt.RegisteredClaims
}

// issueNewSessionTokenPair 为一次新的登录会话签发双 token。
// 第一次登录、注册后自动登录都会走这个入口，它会新建 session_id。
func issueNewSessionTokenPair(svcCtx *svc.ServiceContext, userID int64, device, loginIP string) (*passportpb.TokenPair, error) {
	sessionID, err := repository.NewOpaqueToken("sid_")
	if err != nil {
		return nil, err
	}

	return issueTokenPairForSession(svcCtx, repository.Session{
		SessionID: sessionID,
		UserID:    userID,
		Device:    strings.TrimSpace(device),
		LoginIP:   strings.TrimSpace(loginIP),
	})
}

// rotateSessionTokenPair 为已有 session 轮换 refresh_token，并重新签发 access_token。
// 刷新成功后，旧 refresh_token 立即失效，旧 access_token 也会因为 access_jti 变化而失效。
func rotateSessionTokenPair(svcCtx *svc.ServiceContext, session repository.Session) (*passportpb.TokenPair, error) {
	return issueTokenPairForSession(svcCtx, session)
}

func issueTokenPairForSession(svcCtx *svc.ServiceContext, session repository.Session) (*passportpb.TokenPair, error) {
	issuer, secret, accessExpire, refreshExpire, err := tokenSettings(svcCtx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	accessJTI, err := repository.NewOpaqueToken("ajti_")
	if err != nil {
		return nil, err
	}
	refreshJTI, err := repository.NewOpaqueToken("rjti_")
	if err != nil {
		return nil, err
	}
	refreshToken, err := repository.NewOpaqueToken("rtk_")
	if err != nil {
		return nil, err
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	session.UpdatedAt = now
	session.AccessJTI = accessJTI
	session.RefreshJTI = refreshJTI
	session.ExpiresAt = now.Add(time.Duration(refreshExpire) * time.Second)

	accessToken, err := buildAccessToken(secret, accessClaims{
		UserID:    session.UserID,
		SessionID: session.SessionID,
		Device:    session.Device,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   strconv.FormatInt(session.UserID, 10),
			ID:        accessJTI,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Duration(accessExpire) * time.Second)),
		},
	})
	if err != nil {
		return nil, err
	}

	// 先持久化 session，再持久化 refresh_token，保证 access_token 校验有状态源可查。
	if err := svcCtx.SessionStore.Put(session); err != nil {
		return nil, err
	}
	if err := svcCtx.TokenStore.PutRefresh(repository.RefreshToken{
		Token:     refreshToken,
		JTI:       refreshJTI,
		UserID:    session.UserID,
		SessionID: session.SessionID,
		ExpiresAt: session.ExpiresAt,
	}); err != nil {
		return nil, err
	}

	return &passportpb.TokenPair{
		AccessToken:                  accessToken,
		RefreshToken:                 refreshToken,
		AccessTokenExpiresInSeconds:  accessExpire,
		RefreshTokenExpiresInSeconds: refreshExpire,
	}, nil
}

func buildAccessToken(secret string, claims accessClaims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// parseAccessToken 解析并校验 access_token 的签名和标准时间声明。
// 为了支持密钥轮换，会先尝试当前密钥，再尝试上一把密钥。
func parseAccessToken(svcCtx *svc.ServiceContext, tokenString string) (*accessClaims, error) {
	issuer, currentSecret, _, _, err := tokenSettings(svcCtx)
	if err != nil {
		return nil, err
	}

	claims, err := tryParseAccessToken(currentSecret, issuer, tokenString)
	if err == nil {
		return claims, nil
	}
	return nil, errors.New("invalid access token")
}

func tryParseAccessToken(secret, issuer, tokenString string) (*accessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &accessClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*accessClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid claims")
	}
	if issuer != "" && !claims.VerifyIssuer(issuer, true) {
		return nil, errors.New("invalid issuer")
	}
	if claims.UserID <= 0 || strings.TrimSpace(claims.SessionID) == "" || strings.TrimSpace(claims.ID) == "" {
		return nil, errors.New("invalid access token payload")
	}
	return claims, nil
}

func tokenSettings(svcCtx *svc.ServiceContext) (issuer, secret string, accessExpire, refreshExpire int64, err error) {
	issuer = strings.TrimSpace(svcCtx.Config.Token.Issuer)
	if issuer == "" {
		issuer = "hfjj-passport"
	}

	secret = strings.TrimSpace(svcCtx.Config.Token.AccessSecret)
	if secret == "" {
		return "", "", 0, 0, errors.New("token.access_secret is empty")
	}

	accessExpire = svcCtx.Config.Token.AccessExpireSeconds
	if accessExpire <= 0 {
		accessExpire = 15 * 60
	}
	refreshExpire = svcCtx.Config.Token.RefreshExpireSeconds
	if refreshExpire <= 0 {
		refreshExpire = 3 * 24 * 60 * 60
	}
	return issuer, secret, accessExpire, refreshExpire, nil
}

// buildUserProfile 把仓储层用户对象转换成 proto 层用户信息。
func buildUserProfile(user repository.User) *passportpb.UserProfile {
	return &passportpb.UserProfile{
		UserId:        user.ID,
		Account:       user.Account,
		Nickname:      user.Nickname,
		AvatarUrl:     user.AvatarURL,
		CoverImageUrl: user.CoverImageURL,
		AccountStatus: passportpb.AccountStatus(user.AccountStatus),
		SchoolBound:   user.SchoolBound,
		Bio:           user.Bio,
		CreatedAt:     user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
