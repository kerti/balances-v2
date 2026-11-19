package clientip_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kerti/balances-v2/backend/internal/clientip"
)

const dockerRanges = "172.16.0.0/12,10.0.0.0/8,192.168.0.0/16"

// request builds a request from remoteAddr carrying the given header values
// (each XFF string is one header line, so repeated headers are expressible).
func request(remoteAddr string, flyIP string, xff ...string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/auth/local/login", nil)
	req.RemoteAddr = remoteAddr
	if flyIP != "" {
		req.Header.Set("Fly-Client-IP", flyIP)
	}
	for _, v := range xff {
		req.Header.Add("X-Forwarded-For", v)
	}
	return req
}

// covers: INV-AUTH-34
func TestClientIP(t *testing.T) {
	cases := []struct {
		name    string
		trusted string
		remote  string
		fly     string
		xff     []string
		want    string
	}{
		// Nothing trusted: the peer keys the request, whatever the headers say.
		{"empty list ignores forged XFF", "", "203.0.113.9:1234", "", []string{"198.51.100.1"}, "203.0.113.9"},
		{"empty list ignores forged Fly-Client-IP", "", "203.0.113.9:1234", "198.51.100.7", nil, "203.0.113.9"},
		{"empty list, no headers", "", "203.0.113.9:1234", "", nil, "203.0.113.9"},

		// Untrusted peer under a CIDR list: forged headers do not change the key.
		{"untrusted peer, forged XFF", dockerRanges, "203.0.113.9:1234", "", []string{"198.51.100.1"}, "203.0.113.9"},
		{"untrusted peer, forged Fly-Client-IP", dockerRanges, "203.0.113.9:1234", "198.51.100.7", nil, "203.0.113.9"},
		{"untrusted peer, fresh forged value per request is the same key", dockerRanges, "203.0.113.9:5555", "198.51.100.8", []string{"198.51.100.99"}, "203.0.113.9"},

		// Trusted peer, one proxy: the XFF hop it appended is the client.
		{"one proxy", dockerRanges, "172.18.0.2:4000", "", []string{"198.51.100.5"}, "198.51.100.5"},
		{"one proxy, caller-forged leftmost ignored", dockerRanges, "172.18.0.2:4000", "", []string{"6.6.6.6, 198.51.100.5"}, "198.51.100.5"},
		{"trusted peer never reads Fly-Client-IP", dockerRanges, "172.18.0.2:4000", "6.6.6.6", []string{"198.51.100.5"}, "198.51.100.5"},
		{"trusted peer, Fly-Client-IP only, falls back to peer", dockerRanges, "172.18.0.2:4000", "6.6.6.6", nil, "172.18.0.2"},

		// Two trusted proxies: skip both, land on the real client.
		{"two proxies, one header", "10.0.0.0/8,172.16.0.0/12", "172.18.0.2:1", "", []string{"198.51.100.5, 10.1.1.1"}, "198.51.100.5"},
		{"two proxies, split over repeated headers", "10.0.0.0/8,172.16.0.0/12", "172.18.0.2:1", "", []string{"6.6.6.6", "198.51.100.5", "10.1.1.1"}, "198.51.100.5"},
		{"two proxies, forged leftmost before the real client", "10.0.0.0/8,172.16.0.0/12", "172.18.0.2:1", "", []string{"6.6.6.6, 7.7.7.7, 198.51.100.5, 10.1.1.1"}, "198.51.100.5"},
		{"bare IP proxies", "172.18.0.2,10.1.1.1", "172.18.0.2:1", "", []string{"198.51.100.5, 10.1.1.1"}, "198.51.100.5"},

		// The walk cannot name an untrusted hop.
		{"trusted peer, no XFF", dockerRanges, "172.18.0.2:4000", "", nil, "172.18.0.2"},
		{"all hops trusted: rightmost, not the forgeable leftmost", dockerRanges, "172.18.0.2:4000", "", []string{"192.168.1.50, 10.9.9.9"}, "10.9.9.9"},
		{"LAN client behind one proxy", dockerRanges, "172.18.0.2:4000", "", []string{"192.168.1.50"}, "192.168.1.50"},
		{"garbage rightmost stops the walk at the peer", dockerRanges, "172.18.0.2:4000", "", []string{"198.51.100.5, not-an-ip"}, "172.18.0.2"},
		{"garbage left of a real client is never reached", dockerRanges, "172.18.0.2:4000", "", []string{"not-an-ip, 198.51.100.5"}, "198.51.100.5"},
		{"empty hop stops the walk", dockerRanges, "172.18.0.2:4000", "", []string{"198.51.100.5, "}, "172.18.0.2"},
		{"all garbage", dockerRanges, "172.18.0.2:4000", "", []string{"unknown"}, "172.18.0.2"},

		// Address spellings collapse to one key.
		{"IPv4-mapped peer is matched as IPv4", "172.16.0.0/12", "[::ffff:172.18.0.2]:4000", "", []string{"198.51.100.5"}, "198.51.100.5"},
		{"IPv4-mapped untrusted peer keys as IPv4", "172.16.0.0/12", "[::ffff:203.0.113.9]:4000", "", []string{"198.51.100.5"}, "203.0.113.9"},
		{"IPv4-mapped client hop keys as IPv4", dockerRanges, "172.18.0.2:4000", "", []string{"::ffff:198.51.100.5"}, "198.51.100.5"},
		{"IPv6 client", dockerRanges, "172.18.0.2:4000", "", []string{"2001:db8::7"}, "2001:db8::7"},
		{"IPv6 peer in an IPv6 CIDR", "fd00::/8", "[fd00::2]:4000", "", []string{"2001:db8::7"}, "2001:db8::7"},
		{"zone on a client hop is dropped", dockerRanges, "172.18.0.2:4000", "", []string{"fe80::1%eth0"}, "fe80::1"},
		{"port on a client hop is dropped", dockerRanges, "172.18.0.2:4000", "", []string{"198.51.100.5:6000"}, "198.51.100.5"},
		{"RemoteAddr without a port", dockerRanges, "203.0.113.9", "", nil, "203.0.113.9"},

		// fly preset: Fly-Client-IP, and only from the 172.16/12 peer.
		{"fly: Fly-Client-IP from Fly private peer", "fly", "172.19.0.2:1", "198.51.100.7", nil, "198.51.100.7"},
		{"fly: Fly-Client-IP wins over XFF", "fly", "172.19.0.2:1", "198.51.100.7", []string{"6.6.6.6, 198.51.100.1"}, "198.51.100.7"},
		{"fly: forged Fly-Client-IP from public peer", "fly", "203.0.113.9:1", "198.51.100.7", nil, "203.0.113.9"},
		{"fly: forged Fly-Client-IP from 10/8 peer is not Fly", "fly", "10.0.0.5:1", "198.51.100.7", nil, "10.0.0.5"},
		{"fly: XFF-only forging from public peer", "fly", "203.0.113.9:1", "", []string{"198.51.100.1"}, "203.0.113.9"},
		{"fly: XFF-only from Fly peer is ignored", "fly", "172.19.0.2:1", "", []string{"198.51.100.1, 198.51.100.2"}, "172.19.0.2"},
		{"fly: invalid Fly-Client-IP falls to peer", "fly", "172.19.0.2:1", "garbage", []string{"198.51.100.1"}, "172.19.0.2"},
		{"fly: IPv4-mapped peer", "fly", "[::ffff:172.19.0.2]:1", "198.51.100.7", nil, "198.51.100.7"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			trusted, err := clientip.Parse(tc.trusted)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tc.trusted, err)
			}
			if got := trusted.ClientIP(request(tc.remote, tc.fly, tc.xff...)); got != tc.want {
				t.Errorf("ClientIP = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	ok := []struct {
		in    string
		empty bool
		fly   bool
	}{
		{"", true, false},
		{"   ", true, false},
		{"fly", false, true},
		{" FLY ", false, true},
		{"10.0.0.0/8", false, false},
		{"10.0.0.1", false, false},
		{"2001:db8::/32, 192.168.0.0/16", false, false},
		{"::1", false, false},
		{"::ffff:10.0.0.0/104", false, false},
	}
	for _, tc := range ok {
		got, err := clientip.Parse(tc.in)
		if err != nil {
			t.Errorf("Parse(%q): unexpected error %v", tc.in, err)
			continue
		}
		if got.Empty() != tc.empty || got.IsFly() != tc.fly {
			t.Errorf("Parse(%q): empty=%v fly=%v, want empty=%v fly=%v", tc.in, got.Empty(), got.IsFly(), tc.empty, tc.fly)
		}
	}

	bad := []struct{ in, mention string }{
		{"10.0.0.0/33", "10.0.0.0/33"},
		{"not-a-cidr", "not-a-cidr"},
		{"10.0.0.0/8,bogus", "bogus"},
		{"10.0.0.0/8,,172.16.0.0/12", "empty entry"},
		{"fly,10.0.0.0/8", "preset"},
		{"10.0.0.0/8,fly", "preset"},
		{"fly-edge", "fly-edge"},
	}
	for _, tc := range bad {
		_, err := clientip.Parse(tc.in)
		if err == nil {
			t.Errorf("Parse(%q): want error, got nil", tc.in)
			continue
		}
		if !strings.Contains(err.Error(), tc.mention) || !strings.Contains(err.Error(), "TRUSTED_PROXIES") {
			t.Errorf("Parse(%q) error %q should name TRUSTED_PROXIES and %q", tc.in, err, tc.mention)
		}
	}
}

// A mapped-IPv4 prefix must match the same hosts as its IPv4 spelling.
func TestParse_MappedPrefixMatchesIPv4(t *testing.T) {
	trusted, err := clientip.Parse("::ffff:10.0.0.0/104")
	if err != nil {
		t.Fatal(err)
	}
	if got := trusted.ClientIP(request("10.1.2.3:1", "", "198.51.100.5")); got != "198.51.100.5" {
		t.Errorf("ClientIP = %q, want the XFF client (peer is inside the mapped prefix)", got)
	}
}
