package ownhttp

import "net"

// LanIPs returns every non-loopback IPv4 address on the host, in
// interface-enumeration order. Multiple values are common when the
// machine is on Wi-Fi + ethernet + a VPN. Used by dev servers to
// print a "reachable from another device on the same network" URL
// at startup.
func LanIPs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []string
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() {
			continue
		}
		ip4 := ipnet.IP.To4()
		if ip4 == nil {
			continue
		}
		out = append(out, ip4.String())
	}
	return out
}
