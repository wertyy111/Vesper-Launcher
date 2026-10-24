package auth

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

type UserProfile struct {
	Username    string `json:"username"`
	UUID        string `json:"uuid"`
	AccessToken string `json:"accessToken"`
	UserType    string `json:"userType"`
	IsOffline   bool   `json:"isOffline"`
}

// CreateOfflineProfile produces an official Mojang-compatible offline UUID and profile.
func CreateOfflineProfile(nickname string) UserProfile {
	trimmed := strings.TrimSpace(nickname)
	if trimmed == "" {
		trimmed = "Steve"
	}

	hash := md5.Sum([]byte("OfflinePlayer:" + trimmed))
	// Set version 3 (name-based MD5) and variant
	hash[6] = (hash[6] & 0x0f) | 0x30
	hash[8] = (hash[8] & 0x3f) | 0x80

	rawHex := hex.EncodeToString(hash[:])
	formattedUUID := fmt.Sprintf("%s-%s-%s-%s-%s",
		rawHex[0:8], rawHex[8:12], rawHex[12:16], rawHex[16:20], rawHex[20:32])

	return UserProfile{
		Username:    trimmed,
		UUID:        formattedUUID,
		AccessToken: "00000000-0000-0000-0000-000000000000",
		UserType:    "mojang",
		IsOffline:   true,
	}
}
