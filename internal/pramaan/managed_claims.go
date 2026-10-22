package pramaan

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"
)

func verifiedManagedClaims(raw, issuer, subject string) (ManagedIdentity, error) {
	fail := errors.New("Managed sign-in was refused. Sign in again.")
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return ManagedIdentity{}, fail
	}
	b, e := base64.RawURLEncoding.DecodeString(parts[1])
	if e != nil {
		return ManagedIdentity{}, fail
	}
	var claims struct {
		Sub         string `json:"sub"`
		Iss         string `json:"iss"`
		Email       string `json:"email"`
		Name        string `json:"name"`
		Verified    bool   `json:"emailVerified"`
		VerifiedAlt bool   `json:"email_verified"`
		Exp         int64  `json:"exp"`
		Audience    any    `json:"aud"`
		Party       string `json:"azp"`
	}
	if json.Unmarshal(b, &claims) != nil || claims.Sub != subject || claims.Iss != issuer || claims.Exp <= time.Now().Unix() {
		return ManagedIdentity{}, fail
	}
	audienceOK := false
	if value, ok := claims.Audience.(string); ok {
		audienceOK = value == issuer
	}
	if list, ok := claims.Audience.([]any); ok {
		for _, value := range list {
			if value == issuer {
				audienceOK = true
			}
		}
	}
	if !audienceOK {
		return ManagedIdentity{}, fail
	}
	address, e := mail.ParseAddress(claims.Email)
	if e != nil || address.Address != claims.Email {
		return ManagedIdentity{}, fail
	}
	sum := sha256.Sum256([]byte(raw))
	return ManagedIdentity{Subject: subject, Issuer: issuer, Email: address.Address, Name: claims.Name, EmailVerified: claims.Verified || claims.VerifiedAlt, TokenHash: hex.EncodeToString(sum[:])}, nil
}
