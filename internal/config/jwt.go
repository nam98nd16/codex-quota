package config

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

func ParseAccessToken(token string) AccessTokenClaims {
	token = strings.TrimSpace(token)
	if token == "" {
		return AccessTokenClaims{}
	}

	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return AccessTokenClaims{}
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AccessTokenClaims{}
	}

	claimsMap := map[string]any{}
	if err := json.Unmarshal(payload, &claimsMap); err != nil {
		return AccessTokenClaims{}
	}

	claims := AccessTokenClaims{
		ClientID: strings.TrimSpace(asString(claimsMap["client_id"])),
		Email:    strings.TrimSpace(asString(claimsMap["email"])),
		UserID:   strings.TrimSpace(asString(claimsMap["user_id"])),
	}
	if claims.ClientID == "" {
		claims.ClientID = strings.TrimSpace(asString(claimsMap["cid"]))
	}
	if claims.ClientID == "" {
		claims.ClientID = strings.TrimSpace(asString(claimsMap["clientId"]))
	}
	if claims.Email == "" {
		if profileMap := asMap(claimsMap["https://api.openai.com/profile"]); profileMap != nil {
			claims.Email = strings.TrimSpace(asString(profileMap["email"]))
		}
	}

	rawAuthAccountID := strings.TrimSpace(asString(claimsMap["https://api.openai.com/auth"]))
	if rawAuthAccountID == "" {
		if authMap := asMap(claimsMap["https://api.openai.com/auth"]); authMap != nil {
			rawAuthAccountID = strings.TrimSpace(asString(authMap["chatgpt_account_id"]))
			if claims.UserID == "" {
				claims.UserID = strings.TrimSpace(asString(authMap["chatgpt_user_id"]))
			}
			if claims.UserID == "" {
				claims.UserID = strings.TrimSpace(asString(authMap["user_id"]))
			}
		}
	}
	rawAccountID := strings.TrimSpace(asString(claimsMap["account_id"]))
	subjectID := strings.TrimSpace(asString(claimsMap["sub"]))
	if claims.UserID == "" {
		claims.UserID = subjectID
	}
	claims.AccountID = CanonicalAccountID(rawAuthAccountID, rawAccountID, subjectID)

	if exp, ok := asInt64(claimsMap["exp"]); ok && exp > 0 {
		claims.ExpiresAt = time.Unix(exp, 0)
	}

	return claims
}
