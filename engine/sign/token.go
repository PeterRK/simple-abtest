package sign

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"time"

	"github.com/peterrk/simple-abtest/utils"
)

func BuildPublicToken(signingSecret string, appID, expireAt uint32) string {
	raw := make([]byte, 24)
	binary.BigEndian.PutUint32(raw[0:4], appID)
	binary.BigEndian.PutUint32(raw[4:8], expireAt)

	mac := hmac.New(sha256.New, utils.UnsafeStringToBytes(signingSecret))
	mac.Write(raw[:8])
	copy(raw[8:], mac.Sum(nil)[:16])
	return base64.RawURLEncoding.EncodeToString(raw)
}

func BuildPublicTokenV2(signingSecret string, appID, expireAt, capability uint32) (string, bool) {
	if capability >= 1<<24 {
		return "", false
	}

	raw := make([]byte, 27)
	binary.BigEndian.PutUint32(raw[0:4], appID)
	binary.BigEndian.PutUint32(raw[4:8], expireAt)
	raw[8] = byte(capability)
	raw[9] = byte(capability >> 8)
	raw[10] = byte(capability >> 16)

	mac := hmac.New(sha256.New, utils.UnsafeStringToBytes(signingSecret))
	mac.Write(raw[:11])
	copy(raw[11:], mac.Sum(nil)[:16])
	return base64.RawURLEncoding.EncodeToString(raw), true
}

func VerifyPublicToken(signingSecret string, appID uint32, raw string) bool {
	_, ok := VerifyPublicTokenV2(signingSecret, appID, raw)
	return ok
}

func VerifyPublicTokenV2(signingSecret string, appID uint32, raw string) (uint32, bool) {
	var capability uint32
	buf, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return 0, false
	}

	payloadSize := 0
	switch len(buf) {
	case 24:
		payloadSize = 8
	case 27:
		payloadSize = 11
		capability = uint32(buf[8]) | uint32(buf[9])<<8 | uint32(buf[10])<<16
	default:
		return 0, false
	}

	tokenAppID := binary.BigEndian.Uint32(buf[0:4])
	if tokenAppID != appID {
		return 0, false
	}
	expireAt := binary.BigEndian.Uint32(buf[4:8])
	if uint32(time.Now().Unix()) > expireAt {
		return 0, false
	}

	mac := hmac.New(sha256.New, utils.UnsafeStringToBytes(signingSecret))
	mac.Write(buf[:payloadSize])
	expected := mac.Sum(nil)[:16]
	return capability, hmac.Equal(expected, buf[payloadSize:])
}
