package pramaan

import (
	"context"
	"net"
	"strconv"
	"strings"
)

type DNSRecord struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Value    string `json:"value"`
	Priority uint16 `json:"priority"`
}
type DNS struct{}

func (DNS) Records(ctx context.Context, host string) ([]DNSRecord, error) {
	var out struct {
		Records []DNSRecord `json:"records"`
	}
	e := Call(ctx, "security.dns.lookup", map[string]any{"domain": strings.TrimSuffix(host, ".")}, &out)
	return out.Records, e
}
func (d DNS) LookupTXT(ctx context.Context, host string) ([]string, error) {
	r, e := d.Records(ctx, host)
	out := []string{}
	for _, v := range r {
		if v.Type == "TXT" {
			out = append(out, strings.Trim(v.Value, "\""))
		}
	}
	return out, e
}
func (d DNS) LookupMX(ctx context.Context, host string) ([]*net.MX, error) {
	r, e := d.Records(ctx, host)
	out := []*net.MX{}
	for _, v := range r {
		if v.Type == "MX" {
			host, pref := v.Value, v.Priority
			fields := strings.Fields(v.Value)
			if len(fields) == 2 {
				if n, err := strconv.ParseUint(fields[0], 10, 16); err == nil {
					pref = uint16(n)
					host = fields[1]
				}
			}
			out = append(out, &net.MX{Host: host, Pref: pref})
		}
	}
	return out, e
}
func (d DNS) LookupCNAME(ctx context.Context, host string) (string, error) {
	r, e := d.Records(ctx, host)
	for _, v := range r {
		if v.Type == "CNAME" {
			return v.Value, e
		}
	}
	return host, e
}
func (d DNS) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	var out struct {
		Addresses []string `json:"addresses"`
	}
	e := Call(ctx, "security.dns.resolve", map[string]any{"domain": strings.TrimSuffix(host, ".")}, &out)
	ips := []net.IPAddr{}
	for _, v := range out.Addresses {
		if ip := net.ParseIP(v); ip != nil {
			ips = append(ips, net.IPAddr{IP: ip})
		}
	}
	return ips, e
}

func (d DNS) LookupSRV(ctx context.Context, service, proto, host string) (string, []*net.SRV, error) {
	name := "_" + service + "._" + proto + "." + strings.TrimSuffix(host, ".")
	records, err := d.Records(ctx, name)
	out := []*net.SRV{}
	for _, r := range records {
		if r.Type != "SRV" {
			continue
		}
		f := strings.Fields(r.Value)
		if len(f) != 4 {
			continue
		}
		a, e1 := strconv.ParseUint(f[0], 10, 16)
		b, e2 := strconv.ParseUint(f[1], 10, 16)
		c, e3 := strconv.ParseUint(f[2], 10, 16)
		if e1 == nil && e2 == nil && e3 == nil {
			out = append(out, &net.SRV{Priority: uint16(a), Weight: uint16(b), Port: uint16(c), Target: f[3]})
		}
	}
	return name, out, err
}
