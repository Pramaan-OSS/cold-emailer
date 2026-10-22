// Package mailcause turns a failed mailbox connect into a stable cause key
// with a short title and a concrete fix, so an import can group its failures
// by what the person has to do about them.
package mailcause

// Cause is one kind of connect failure and what fixes it.
type Cause struct {
	Key   string `json:"cause"`
	Title string `json:"title"`
	Fix   string `json:"fix"`
	// Retryable: the same settings can succeed on a later attempt or with a new password.
	Retryable bool `json:"retryable"`
}

// Cause keys.
const (
	GoogleAppPasswordRequired   = "google_app_password_required"
	GoogleBadCredentials        = "google_bad_credentials"
	GoogleIMAPDisabled          = "google_imap_disabled"
	GoogleWebLoginRequired      = "google_web_login_required"
	MicrosoftSMTPAuthDisabled   = "microsoft_smtp_auth_disabled"
	MicrosoftSecurityDefaults   = "microsoft_security_defaults"
	MicrosoftConditionalAccess  = "microsoft_conditional_access"
	MicrosoftBasicAuthDisabled  = "microsoft_basic_auth_disabled"
	MicrosoftBadCredentials     = "microsoft_bad_credentials"
	YahooAppPasswordRequired    = "yahoo_app_password_required"
	AOLAppPasswordRequired      = "aol_app_password_required"
	ICloudAppPasswordRequired   = "icloud_app_password_required"
	ZohoAppPasswordRequired     = "zoho_app_password_required"
	FastmailAppPasswordRequired = "fastmail_app_password_required"
	YandexAppPasswordRequired   = "yandex_app_password_required"
	IMAPDisabled                = "imap_disabled"
	AccountLocked               = "account_locked"
	TooManyLogins               = "too_many_logins"
	AuthUnavailable             = "auth_unavailable"
	AuthRefused                 = "auth_refused"
	HostNotFound                = "host_not_found"
	HostUnreachable             = "host_unreachable"
	PortBlocked                 = "port_blocked"
	TLSFailed                   = "tls_failed"
	CleartextRefused            = "cleartext_refused"
	Timeout                     = "timeout"
	Temporary                   = "temporary"
	ServerDeclined              = "server_declined"
)

var catalogue = []Cause{
	{GoogleAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{GoogleBadCredentials, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{GoogleIMAPDisabled, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{GoogleWebLoginRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{MicrosoftSMTPAuthDisabled, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{MicrosoftSecurityDefaults, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{MicrosoftConditionalAccess, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{MicrosoftBasicAuthDisabled, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", false},
	{MicrosoftBadCredentials, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{YahooAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{AOLAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{ICloudAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{ZohoAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{FastmailAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{YandexAppPasswordRequired, "Mailbox connection unavailable", "Connect or reconnect this mailbox in Connections. Raw protocol diagnostics are unavailable.", true},
	{IMAPDisabled, "IMAP is turned off for this mailbox",
		"The server says IMAP access is disabled for this account. Turn IMAP on in the provider's mail settings (or ask your email admin to), then retry.", true},
	{AccountLocked, "The account is locked or disabled",
		"The provider says this account is locked, suspended or disabled. Sign in to it in a browser to see why and restore access, or ask your email admin, then retry.", false},
	{TooManyLogins, "Too many sign-in attempts",
		"The provider is limiting sign-ins for this account or from this network. Wait 15 to 60 minutes before retrying, and avoid retrying the same failing password repeatedly.", true},
	{AuthUnavailable, "The server does not take a password here",
		"The server did not offer password sign-in on this port. Check the host and port: use the provider's submission server (usually port 587 or 465), not the MX server or port 25.", false},
	{AuthRefused, "The server refused the sign-in",
		"The server refused this username and password. Check both, and whether the provider needs an app password or a username other than the email address.", true},
	{HostNotFound, "The server name does not exist",
		"The server hostname could not be found in DNS. Check the spelling of the SMTP and IMAP host.", false},
	{HostUnreachable, "The server could not be reached",
		"No connection could be made to the server. Check the host and port, and that the server accepts connections from outside your network.", false},
	{PortBlocked, "The mail port looks blocked",
		"The SMTP port did not answer while IMAP worked, which usually means the port is blocked on the way. Try port 587 with STARTTLS instead of 465 or 25, or check the provider's documented SMTP port.", false},
	{TLSFailed, "The secure connection failed",
		"The server did not complete a secure connection. Check the security setting for the port: 465 and 993 use SSL/TLS, 587 and 143 use STARTTLS. Also check that the hostname matches the server's certificate.", false},
	{CleartextRefused, "Encryption is required",
		"The connection was attempted without encryption, which is not allowed here. Use SSL/TLS on 465 or 993, or STARTTLS on 587 or 143.", false},
	{Timeout, "The server did not answer in time",
		"The server did not answer before the time ran out. It may be slow or overloaded; retry later, and check the host and port if it keeps happening.", true},
	{Temporary, "The server asked to retry later",
		"The server declined the sign-in for now and asked for a retry. Retry in a few minutes.", true},
	{ServerDeclined, "The server did not accept the connection",
		"The server answered but did not accept the sign-in. Check the host, port, security setting and credentials, then retry.", true},
}

var byKey = func() map[string]Cause {
	m := make(map[string]Cause, len(catalogue))
	for _, c := range catalogue {
		m[c.Key] = c
	}
	return m
}()

// Lookup returns the catalogue entry for key.
func Lookup(key string) (Cause, bool) {
	c, ok := byKey[key]
	return c, ok
}

// Keys lists every cause key in catalogue order.
func Keys() []string {
	out := make([]string, len(catalogue))
	for i, c := range catalogue {
		out[i] = c.Key
	}
	return out
}

// Generic reports whether key is a catch-all a finer classifier may refine.
func Generic(key string) bool {
	return key == AuthRefused || key == ServerDeclined
}

func must(key string) Cause {
	c, ok := byKey[key]
	if !ok {
		return byKey[ServerDeclined]
	}
	return c
}
