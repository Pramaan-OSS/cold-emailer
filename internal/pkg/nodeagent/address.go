package nodeagent

import (
	"context"
	"net/netip"
)

func (a *Agent) publicAddress(ctx context.Context) string {
	ip, err := netip.ParseAddr(a.cfg.Address)
	if err != nil {
		return ""
	}
	return ip.Unmap().String()
}
