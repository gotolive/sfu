package ice

import (
	"net"
)

// getInterfaceIps returns a slice of IP addresses for the network interfaces on the machine
func getInterfaceIps(v6 bool) ([]string, error) {
	var ips []string
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		// Check if the address is an IP address
		if ipNet, ok := addr.(*net.IPNet); ok {
			// Check if the address is an IPv4 or IPv6 address based on the v6 parameter
			if (ipNet.IP.To4() != nil) || (v6 && isSupportedIPv6(ipNet.IP)) {
				ips = append(ips, ipNet.IP.String())
			}
		}
	}
	// Check if any IP addresses were added to the slice
	if len(ips) == 0 {
		return nil, ErrNoAvailableIP
	}

	return ips, nil
}

// from pion/ice
// The conditions of invalidation written below are defined in
// https://tools.ietf.org/html/rfc8445#section-5.1.1.1
func isSupportedIPv6(ip net.IP) bool {
	if len(ip) != net.IPv6len ||
		isZeros(ip[0:12]) || // !(IPv4-compatible IPv6)
		ip[0] == 0xfe && ip[1]&0xc0 == 0xc0 || // !(IPv6 site-local unicast)
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() {
		return false
	}
	return true
}

func isZeros(ip net.IP) bool {
	for i := 0; i < len(ip); i++ {
		if ip[i] != 0 {
			return false
		}
	}
	return true
}
