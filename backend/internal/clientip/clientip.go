// Package clientip decides which address keys the login/reset rate limiter
// (#363, #755). A forwarding header is only evidence when the host that sent it
// is one the operator vouched for, so the TCP peer is checked first and every
// header is ignored unless the peer is inside the configured TRUSTED_PROXIES.
package clientip

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// flyPreset is the TRUSTED_PROXIES literal for the Fly.io deployment.
const flyPreset = "fly"

// flyPrivate is Fly's private network, where its edge reaches the app from.
// Under the fly preset a peer in this range is the only source the
// Fly-Client-IP header is believed from.
var flyPrivate = netip.MustParsePrefix("172.16.0.0/12")

// Trusted is the parsed TRUSTED_PROXIES setting. The zero value trusts
// nothing: every request is keyed on its TCP peer.
type Trusted struct {
	fly  bool
	nets []netip.Prefix
}

// Parse reads a TRUSTED_PROXIES value: a comma-separated list of CIDRs (a bare
// IP is a /32 or /128), or the literal `fly` on its own. Blank means trust
// nothing. An invalid entry is an error that names it.
func Parse(s string) (Trusted, error) {
	var t Trusted
	if strings.TrimSpace(s) == "" {
		return t, nil
	}
	entries := strings.Split(s, ",")
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		switch {
		case entry == "":
			return Trusted{}, fmt.Errorf("TRUSTED_PROXIES: empty entry in %q", s)
		case strings.EqualFold(entry, flyPreset):
			if len(entries) != 1 {
				return Trusted{}, fmt.Errorf("TRUSTED_PROXIES: %q is a preset and cannot be combined with other entries", flyPreset)
			}
			t.fly = true
		default:
			p, err := parseEntry(entry)
			if err != nil {
				return Trusted{}, fmt.Errorf("TRUSTED_PROXIES: invalid entry %q (want a CIDR such as 10.0.0.0/8, a bare IP, or %q)", entry, flyPreset)
			}
			t.nets = append(t.nets, p)
		}
	}
	return t, nil
}

func parseEntry(entry string) (netip.Prefix, error) {
	if strings.Contains(entry, "/") {
		p, err := netip.ParsePrefix(entry)
		if err != nil {
			return netip.Prefix{}, err
		}
		// An IPv4-mapped prefix (::ffff:10.0.0.0/104) is the IPv4 range spelled
		// the long way; contains() unmaps addresses, so unmap the prefix too.
		if p.Addr().Is4In6() && p.Bits() >= 96 {
			return netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96).Masked(), nil
		}
		return p.Masked(), nil
	}
	a, err := netip.ParseAddr(entry)
	if err != nil {
		return netip.Prefix{}, err
	}
	a = a.Unmap().WithZone("")
	return netip.PrefixFrom(a, a.BitLen()), nil
}

// IsFly reports whether the fly preset is selected.
func (t Trusted) IsFly() bool { return t.fly }

// Empty reports whether nothing is trusted.
func (t Trusted) Empty() bool { return !t.fly && len(t.nets) == 0 }

func (t Trusted) contains(a netip.Addr) bool {
	if t.fly {
		return flyPrivate.Contains(a)
	}
	for _, p := range t.nets {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// ClientIP returns the key address for r.
//
//   - Peer not trusted (or nothing configured): the peer. No header is read.
//   - fly preset, peer in 172.16.0.0/12: a valid Fly-Client-IP, else the peer.
//     X-Forwarded-For is never consulted — Fly's edge appends its own ingress
//     address after the visitor, so the rightmost hop is not the client.
//   - CIDR list, trusted peer: every X-Forwarded-For value (repeated headers and
//     comma lists alike) is walked right to left, skipping trusted hops; the
//     first untrusted address is the client. Fly-Client-IP is never read.
//
// The XFF walk ends in the peer, not the leftmost entry, when it cannot name an
// untrusted hop. An unparseable entry stops the walk: whatever sits left of it
// is not something we can vouch for, and the peer is a key no caller chooses.
// When every hop is trusted (a client inside a trusted range, e.g. a LAN
// address behind a private-range preset) the answer is the rightmost entry —
// the one the proxy nearest the app wrote itself — never the leftmost, which is
// the entry a caller on a trusted network is free to forge into a fresh key per
// request.
func (t Trusted) ClientIP(r *http.Request) string {
	peer, ok := parsePeer(r.RemoteAddr)
	if !ok {
		return r.RemoteAddr
	}
	if !t.contains(peer) {
		return peer.String()
	}
	if t.fly {
		if a, ok := parseHop(r.Header.Get("Fly-Client-IP")); ok {
			return a.String()
		}
		return peer.String()
	}

	var hops []string
	for _, v := range r.Header.Values("X-Forwarded-For") {
		hops = append(hops, strings.Split(v, ",")...)
	}
	var rightmost netip.Addr
	for i := len(hops) - 1; i >= 0; i-- {
		a, ok := parseHop(hops[i])
		if !ok {
			return peer.String()
		}
		if !t.contains(a) {
			return a.String()
		}
		if !rightmost.IsValid() {
			rightmost = a
		}
	}
	if rightmost.IsValid() {
		return rightmost.String()
	}
	return peer.String()
}

// parsePeer reads RemoteAddr (host:port, or a bare address in tests).
func parsePeer(remote string) (netip.Addr, bool) {
	if ap, err := netip.ParseAddrPort(remote); err == nil {
		return ap.Addr().Unmap().WithZone(""), true
	}
	if a, err := netip.ParseAddr(remote); err == nil {
		return a.Unmap().WithZone(""), true
	}
	return netip.Addr{}, false
}

// parseHop reads one forwarded address. The zone is dropped (a caller varying
// `fe80::1%x` would otherwise mint a new key per request), an IPv4-mapped IPv6
// form collapses to its IPv4, and a port some proxies append is ignored.
func parseHop(s string) (netip.Addr, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return netip.Addr{}, false
	}
	if a, err := netip.ParseAddr(s); err == nil {
		return a.Unmap().WithZone(""), true
	}
	if ap, err := netip.ParseAddrPort(s); err == nil {
		return ap.Addr().Unmap().WithZone(""), true
	}
	return netip.Addr{}, false
}
