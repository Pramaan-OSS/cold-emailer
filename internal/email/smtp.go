package email

import "context"

func VerifySMTP(context.Context, string, int, string, string, string) ProbeResult {
	return ProbeResult{Reason: "unavailable", Detail: "Raw protocol diagnostics are unavailable. Connect your mailbox through Pramaan."}
}
