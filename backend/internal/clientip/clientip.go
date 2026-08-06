// Package clientip resolves request addresses at the reverse-proxy trust boundary.
package clientip

import (
	"net"
	"net/netip"
	"strings"
)

const (
	maxForwardedBytes = 4096
	maxChainAddresses = 32
)

// Resolve returns a canonical client IP. Invalid forwarding data always falls back
// to the direct TCP peer.
func Resolve(remoteAddr, forwarded string, trusted []string) string {
	peer, ok := parseRemote(remoteAddr)
	if !ok || !trustedPeer(peer, trusted) {
		return peer.String()
	}
	chain, ok := parseChain(forwarded)
	if !ok {
		return peer.String()
	}
	current := peer
	for i := len(chain) - 1; i >= 0; i-- {
		if !trustedPeer(current, trusted) {
			return current.String()
		}
		current = chain[i]
	}
	if !trustedPeer(current, trusted) {
		return current.String()
	}
	return peer.String()
}

func parseRemote(value string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(value)
	if err != nil {
		host = value
	}
	addr, err := netip.ParseAddr(strings.TrimSpace(host))
	return addr.Unmap(), err == nil
}

func parseChain(value string) ([]netip.Addr, bool) {
	if value == "" {
		return nil, false
	}
	if len(value) > maxForwardedBytes {
		return nil, false
	}
	parts := strings.Split(value, ",")
	if len(parts) > maxChainAddresses {
		return nil, false
	}
	chain := make([]netip.Addr, 0, len(parts))
	for _, part := range parts {
		addr, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil {
			return nil, false
		}
		chain = append(chain, addr.Unmap())
	}
	return chain, true
}

func trustedPeer(peer netip.Addr, entries []string) bool {
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if prefix, err := netip.ParsePrefix(entry); err == nil && prefix.Contains(peer) {
			return true
		}
		if addr, err := netip.ParseAddr(entry); err == nil && addr.Unmap() == peer {
			return true
		}
	}
	return false
}
