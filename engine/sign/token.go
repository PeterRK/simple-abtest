package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/peterrk/simple-abtest/utils"
)

const (
	publicTokenPayloadSize   = 8
	publicTokenSignatureSize = 16
	publicTokenRawSize       = publicTokenPayloadSize + publicTokenSignatureSize
)

func BuildPublicToken(signingSecret string, appID, expireAt uint32) string {
	raw := make([]byte, publicTokenRawSize)
	binary.BigEndian.PutUint32(raw[0:4], appID)
	binary.BigEndian.PutUint32(raw[4:8], expireAt)

	mac := hmac.New(sha256.New, utils.UnsafeStringToBytes(signingSecret))
	mac.Write(raw[:publicTokenPayloadSize])
	copy(raw[publicTokenPayloadSize:], mac.Sum(nil)[:publicTokenSignatureSize])
	return base64.RawURLEncoding.EncodeToString(raw)
}

func VerifyPublicToken(signingSecret string, appID uint32, raw string) bool {
	buf, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(buf) != publicTokenRawSize {
		return false
	}

	tokenAppID := binary.BigEndian.Uint32(buf[0:4])
	if tokenAppID != appID {
		return false
	}
	expireAt := binary.BigEndian.Uint32(buf[4:8])
	if uint32(time.Now().Unix()) > expireAt {
		return false
	}

	mac := hmac.New(sha256.New, utils.UnsafeStringToBytes(signingSecret))
	mac.Write(buf[:publicTokenPayloadSize])
	expected := mac.Sum(nil)[:publicTokenSignatureSize]
	return hmac.Equal(expected, buf[publicTokenPayloadSize:])
}
