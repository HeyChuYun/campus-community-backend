package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"hfjj/backend/gateway/internal/svc"
)

type initCampusPasswordKeyRequest struct {
	StudentNo string `json:"student_no"`
}

type exchangeCampusPasswordKeyRequest struct {
	StudentNo       string `json:"student_no"`
	CredentialToken string `json:"credential_token"`
}

type campusPasswordCredentialPayload struct {
	UserID    int64  `json:"user_id"`
	StudentNo string `json:"student_no"`
	Device    string `json:"device"`
	Key       string `json:"key"`
	IssuedAt  int64  `json:"issued_at"`
	ExpiresAt int64  `json:"expires_at"`
}

func initCampusPasswordKeyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req initCampusPasswordKeyRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		// 第一步：确认当前请求已经带上有效的平台 access_token。
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		studentNo := strings.TrimSpace(req.StudentNo)
		if studentNo == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "student_no 不能为空",
			})
			return
		}

		// 第二步：只允许为当前账号已绑定的学号申请本地密码加密密钥。
		account, exists, err := queryGatewaySchoolAccountByUserID(svcCtx, user.GetUserId())
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists || strings.TrimSpace(account.StudentNo) == "" {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    2005,
				Message: "当前账号尚未绑定校园学号",
			})
			return
		}
		if account.StudentNo != studentNo {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "当前账号只能为已绑定学号申请密钥",
			})
			return
		}

		// 第三步：为当前用户和当前设备生成一份新的本地密码保护凭证。
		issued, err := issueCampusPasswordCredential(
			svcCtx,
			user.GetUserId(),
			studentNo,
			clientDevice(r),
		)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    gatewayCodeUpstreamInvalidResponse,
				Message: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"credential_token": issued.Token,
				"encrypt_key":      issued.Key,
			},
		})
	}
}

func exchangeCampusPasswordKeyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req exchangeCampusPasswordKeyRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		// 第一步：确认当前请求已经带上有效的平台 access_token。
		user, ok := verifyBearerToken(w, r, svcCtx)
		if !ok || user == nil {
			return
		}

		studentNo := strings.TrimSpace(req.StudentNo)
		credentialToken := strings.TrimSpace(req.CredentialToken)
		if studentNo == "" || credentialToken == "" {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "student_no 和 credential_token 不能为空",
			})
			return
		}

		// 第二步：只允许轮换当前账号已绑定学号对应的本地密码保护凭证。
		account, exists, err := queryGatewaySchoolAccountByUserID(svcCtx, user.GetUserId())
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{Code: gatewayCodeRPCError, Message: err.Error()})
			return
		}
		if !exists || strings.TrimSpace(account.StudentNo) == "" {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    2005,
				Message: "当前账号尚未绑定校园学号",
			})
			return
		}
		if account.StudentNo != studentNo {
			writeJSON(w, http.StatusBadRequest, jsonResponse{
				Code:    gatewayCodeInvalidArgument,
				Message: "当前账号只能轮换已绑定学号的密钥",
			})
			return
		}

		// 第三步：解开前一次的凭证令牌，并校验用户、学号、设备和有效期。
		payload, err := decodeCampusPasswordCredential(svcCtx, credentialToken)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    2004,
				Message: "校园密码本地凭证已失效，请重新进行校园授权",
			})
			return
		}
		if payload.UserID != user.GetUserId() ||
			payload.StudentNo != studentNo ||
			payload.Device != clientDevice(r) {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    2004,
				Message: "校园密码本地凭证与当前账号或设备不匹配，请重新进行校园授权",
			})
			return
		}

		// 第四步：下发旧密钥用于本地解密，同时生成下一轮的新密钥和新凭证。
		nextIssued, err := issueCampusPasswordCredential(
			svcCtx,
			user.GetUserId(),
			studentNo,
			clientDevice(r),
		)
		if err != nil {
			writeJSON(w, http.StatusOK, jsonResponse{
				Code:    gatewayCodeUpstreamInvalidResponse,
				Message: err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusOK, jsonResponse{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"decrypt_key":           payload.Key,
				"next_credential_token": nextIssued.Token,
				"next_encrypt_key":      nextIssued.Key,
			},
		})
	}
}

type issuedCampusPasswordCredential struct {
	Token string
	Key   string
}

func issueCampusPasswordCredential(
	svcCtx *svc.ServiceContext,
	userID int64,
	studentNo string,
	device string,
) (*issuedCampusPasswordCredential, error) {
	key, err := randomCampusPasswordKey()
	if err != nil {
		return nil, err
	}

	ttl := svcCtx.Config.CampusCredential.TokenTTL
	if ttl <= 0 {
		ttl = 90 * 24 * time.Hour
	}

	now := time.Now()
	payload := campusPasswordCredentialPayload{
		UserID:    userID,
		StudentNo: studentNo,
		Device:    device,
		Key:       key,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	token, err := encodeCampusPasswordCredential(svcCtx, payload)
	if err != nil {
		return nil, err
	}

	return &issuedCampusPasswordCredential{
		Token: token,
		Key:   key,
	}, nil
}

func randomCampusPasswordKey() (string, error) {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func encodeCampusPasswordCredential(
	svcCtx *svc.ServiceContext,
	payload campusPasswordCredentialPayload,
) (string, error) {
	plain, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(deriveCampusCredentialMasterKey(svcCtx))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	encrypted := gcm.Seal(nil, nonce, plain, nil)
	combined := append(nonce, encrypted...)
	return base64.RawURLEncoding.EncodeToString(combined), nil
}

func decodeCampusPasswordCredential(
	svcCtx *svc.ServiceContext,
	token string,
) (*campusPasswordCredentialPayload, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(deriveCampusCredentialMasterKey(svcCtx))
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, io.ErrUnexpectedEOF
	}

	nonce := raw[:gcm.NonceSize()]
	cipherText := raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherText, nil)
	if err != nil {
		return nil, err
	}

	var payload campusPasswordCredentialPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, err
	}
	if payload.UserID <= 0 ||
		strings.TrimSpace(payload.StudentNo) == "" ||
		strings.TrimSpace(payload.Key) == "" ||
		payload.ExpiresAt <= time.Now().Unix() {
		return nil, io.ErrUnexpectedEOF
	}
	return &payload, nil
}

func deriveCampusCredentialMasterKey(svcCtx *svc.ServiceContext) []byte {
	masterKey := strings.TrimSpace(svcCtx.Config.CampusCredential.MasterKey)
	if masterKey == "" {
		masterKey = "hfjj-campus-credential-default-master-key"
	}
	sum := sha256.Sum256([]byte(masterKey))
	return sum[:]
}
