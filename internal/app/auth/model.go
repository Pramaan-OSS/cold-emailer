package auth

import (
	"github.com/pramaan-oss/cold-email/internal/models"
	"time"
)

type AuthData struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	BotCheckToken string `json:"bot_check_token"`
	// ReferralCode is the optional ?ref= code the signup arrived with. Carried
	// to RegistrationConfirm so the new org can be attributed to its referrer.
	ReferralCode string `json:"referral_code"`
	// Invite is the team-invitation token the signup arrived with. It is what
	// lets an invited person create an account under DISABLE_REGISTRATION=invite_only.
	Invite string `json:"invite"`
	// Acquisition is where the visitor came from, read by the dashboard from
	// the signup URL's query string. Recorded once on the new organization and
	// never updated. Absent for a direct visit, which is most signups.
	Acquisition models.OrgAcquisition `json:"acquisition"`
	// DeviceToken is what LoginConfirm returned when the user asked to
	// remember this browser. Login only: it skips the emailed code.
	DeviceToken string `json:"device_token"`
}

type ConfirmData struct {
	Session       string `json:"session"`
	Code          string `json:"code"`
	BotCheckToken string `json:"bot_check_token"`
	// RememberDevice asks for a trusted-device token with the session, so the
	// next sign-in from this browser skips the emailed code. Login only.
	RememberDevice bool `json:"remember_device"`
}

type ResetPasswordStart struct {
	Email         string `json:"email"`
	BotCheckToken string `json:"bot_check_token"`
}

type ResetPasswordConfirm struct {
	// Session is the reset session POST /auth/reset-password/session returned,
	// or the emailed link token itself from a client that predates that step.
	Session  string `json:"session"`
	Password string `json:"password"`
	// RevokeAPICredentials also ends the user's app authorizations and the API keys they created.
	RevokeAPICredentials bool   `json:"revoke_api_credentials,omitempty"`
	BotCheckToken        string `json:"bot_check_token"`
}

// ResetPasswordExchange is the emailed link token, presented in a body so the
// reset session it opens never travels in a URL.
type ResetPasswordExchange struct {
	Token string `json:"token"`
}

// ResetPasswordSession is the reset session a link opens, for the confirm step.
type ResetPasswordSession struct {
	Session   string    `json:"session"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ChangePassword struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	// RevokeAPICredentials also ends the user's app authorizations and the API keys they created.
	RevokeAPICredentials bool `json:"revoke_api_credentials,omitempty"`
}

// SSOLinkData is what POST /auth/sso/link takes: the challenge a federated
// sign-in came back with, and the password of the account it resolved to.
type SSOLinkData struct {
	PendingToken string `json:"pending_token"`
	Password     string `json:"password"`
}
