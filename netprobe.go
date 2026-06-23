package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

func main() {
	var host string
	var port string
	var timeout = 3 * time.Second
	var count = 1
	var showCert bool
	var showTrace bool
	var ipVersion = "ip"

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "-p" || arg == "--port" {
			if i+1 < len(args) {
				port = args[i+1]
				i++
			} else {
				fmt.Println("Error: flag -p/--port requires an argument")
				os.Exit(1)
			}
		} else if arg == "-t" || arg == "--timeout" {
			if i+1 < len(args) {
				val := args[i+1]
				i++
				d, err := time.ParseDuration(val)
				if err == nil {
					timeout = d
				} else {
					// Try parsing as integer seconds
					var sec int
					_, err := fmt.Sscanf(val, "%d", &sec)
					if err == nil {
						timeout = time.Duration(sec) * time.Second
					} else {
						fmt.Printf("Error parsing timeout: %v\n", err)
						os.Exit(1)
					}
				}
			} else {
				fmt.Println("Error: flag -t/--timeout requires an argument")
				os.Exit(1)
			}
		} else if arg == "-c" || arg == "--count" {
			if i+1 < len(args) {
				val := args[i+1]
				i++
				_, err := fmt.Sscanf(val, "%d", &count)
				if err != nil || count <= 0 {
					fmt.Printf("Error: invalid count: %s\n", val)
					os.Exit(1)
				}
			} else {
				fmt.Println("Error: flag -c/--count requires an argument")
				os.Exit(1)
			}
		} else if arg == "-4" {
			ipVersion = "ip4"
		} else if arg == "-6" {
			ipVersion = "ip6"
		} else if arg == "--cert" {
			showCert = true
		} else if arg == "--trace" || arg == "--traceroute" {
			showTrace = true
		} else if arg == "-h" || arg == "--help" {
			printUsage()
			os.Exit(0)
		} else {
			// Positional argument: host
			if host == "" {
				host = arg
			} else {
				fmt.Printf("Error: unexpected argument '%s'\n", arg)
				printUsage()
				os.Exit(1)
			}
		}
	}

	if host == "" {
		printUsage()
		os.Exit(1)
	}

	ip, err := net.ResolveIPAddr(ipVersion, host)
	if err != nil {
		fmt.Printf("Cannot resolve IP address for host '%s': %v\n", host, err)
		os.Exit(1)
	}

	if showTrace {
		run_traceroute(host, ip, timeout)
		return
	}

	if showCert {
		if port == "" {
			port = "443"
		}
		var rtts []time.Duration
		var successCount int
		for i := 0; i < count; i++ {
			success, duration := print_certificate(host, ip, port, timeout)
			if success {
				successCount++
				rtts = append(rtts, duration)
			}
			if i < count-1 {
				time.Sleep(time.Second)
			}
		}
		if count > 1 {
			printSummary(host, count, successCount, rtts)
		}
		if successCount == 0 {
			os.Exit(1)
		}
		return
	}

	if port != "" {
		var tcpSuccess bool
		var rtts []time.Duration
		var successCount int
		for i := 0; i < count; i++ {
			success, duration := tcp_ping(host, ip, port, timeout)
			if success {
				tcpSuccess = true
				successCount++
				rtts = append(rtts, duration)
			}
			if i < count-1 {
				time.Sleep(time.Second)
			}
		}

		if !tcpSuccess {
			fmt.Println("Falling back to ICMP ping...")
			var icmpSuccess bool
			var icmpRtts []time.Duration
			var icmpSuccessCount int
			for i := 0; i < count; i++ {
				success, duration := icmp_ping(host, ip, timeout)
				if success {
					icmpSuccess = true
					icmpSuccessCount++
					icmpRtts = append(icmpRtts, duration)
				}
				if i < count-1 {
					time.Sleep(time.Second)
				}
			}
			if count > 1 {
				printSummary(host, count, icmpSuccessCount, icmpRtts)
			}
			if !icmpSuccess {
				os.Exit(1)
			}
		} else {
			if count > 1 {
				printSummary(host, count, successCount, rtts)
			}
		}
	} else {
		var icmpSuccess bool
		var rtts []time.Duration
		var successCount int
		for i := 0; i < count; i++ {
			success, duration := icmp_ping(host, ip, timeout)
			if success {
				icmpSuccess = true
				successCount++
				rtts = append(rtts, duration)
			}
			if i < count-1 {
				time.Sleep(time.Second)
			}
		}
		if count > 1 {
			printSummary(host, count, successCount, rtts)
		}
		if !icmpSuccess {
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println("Usage: netprobe <host> [options]")
	fmt.Println("\nOptions:")
	fmt.Println("  -p, --port <port>       TCP port to check (e.g., 80, 443)")
	fmt.Println("  -t, --timeout <timeout> Timeout duration (e.g., 3s, 5)")
	fmt.Println("  -c, --count <count>     Number of attempts to perform (default: 1)")
	fmt.Println("  -4                      Force IPv4 resolution")
	fmt.Println("  -6                      Force IPv6 resolution")
	fmt.Println("  --cert                  Inspect TLS certificate details (defaults to port 443)")
	fmt.Println("  --trace, --traceroute   Trace route to the host")
	fmt.Println("  -h, --help              Show this help message")
}

// formatDuration formats duration into a user-friendly string
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%.1fµs", float64(d)/float64(time.Microsecond))
	}
	if d < time.Second {
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2fs", float64(d)/float64(time.Second))
}

// tcp_ping attempts a TCP connection to the destination IP and port
func tcp_ping(host string, ip *net.IPAddr, port string, timeout time.Duration) (bool, time.Duration) {
	d := net.Dialer{Timeout: timeout}
	var hostStr string
	if host == ip.String() {
		hostStr = ip.String()
	} else {
		hostStr = fmt.Sprintf("%s (%s)", host, ip.String())
	}

	start := time.Now()
	conn, err := d.Dial("tcp", net.JoinHostPort(ip.String(), port))
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("TCP connection to %s:%s unsuccessful: %v\n", hostStr, port, err)
		return false, duration
	}
	defer conn.Close()

	fmt.Printf("TCP connection to %s:%s successful - RTT: %s\n", hostStr, port, formatDuration(duration))
	return true, duration
}

// icmp_ping attempts an unprivileged ICMP ping using udp4/udp6 socket
func icmp_ping(host string, ip *net.IPAddr, timeout time.Duration) (bool, time.Duration) {
	isIPv6 := ip.IP.To4() == nil
	network := "udp4"
	proto := 1 // ICMPv4 protocol number
	msgType := icmp.Type(ipv4.ICMPTypeEcho)
	listenAddr := "0.0.0.0"

	if isIPv6 {
		network = "udp6"
		proto = 58 // ICMPv6 protocol number
		msgType = ipv6.ICMPTypeEchoRequest
		listenAddr = "::"
	}

	conn, err := icmp.ListenPacket(network, listenAddr)
	if err != nil {
		fmt.Printf("ICMP ping error: failed to listen on socket: %v\n", err)
		return false, 0
	}
	defer conn.Close()

	var hostStr string
	if host == ip.String() {
		hostStr = ip.String()
	} else {
		hostStr = fmt.Sprintf("%s (%s)", host, ip.String())
	}

	msg := icmp.Message{
		Type: msgType,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("ping"),
		},
	}
	msg_bytes, err := msg.Marshal(nil)
	if err != nil {
		fmt.Printf("ICMP ping error: failed to marshal message: %v\n", err)
		return false, 0
	}

	start := time.Now()
	dst := &net.UDPAddr{IP: ip.IP}
	if _, err := conn.WriteTo(msg_bytes, dst); err != nil {
		fmt.Printf("ICMP ping error: failed to send packet: %v\n", err)
		return false, 0
	}

	err = conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		fmt.Printf("ICMP ping error: failed to set timeout: %v\n", err)
		return false, 0
	}

	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("ICMP ping to %s timed out or failed: %v\n", hostStr, err)
		return false, duration
	}

	parsed_reply, err := icmp.ParseMessage(proto, reply[:n])
	if err != nil {
		fmt.Printf("ICMP ping error: failed to parse reply: %v\n", err)
		return false, duration
	}

	if isIPv6 {
		switch parsed_reply.Type {
		case ipv6.ICMPTypeEchoReply:
			fmt.Printf("ICMP ping to %s successful - RTT: %s\n", hostStr, formatDuration(duration))
			return true, duration
		case ipv6.ICMPTypeDestinationUnreachable:
			fmt.Printf("ICMP ping to %s failed: Destination Unreachable\n", hostStr)
			return false, duration
		case ipv6.ICMPTypeTimeExceeded:
			fmt.Printf("ICMP ping to %s failed: Time Exceeded\n", hostStr)
			return false, duration
		default:
			fmt.Printf("ICMP ping to %s unsuccessful (ICMP type: %v, code: %d)\n", hostStr, parsed_reply.Type, parsed_reply.Code)
			return false, duration
		}
	} else {
		switch parsed_reply.Type {
		case ipv4.ICMPTypeEchoReply:
			fmt.Printf("ICMP ping to %s successful - RTT: %s\n", hostStr, formatDuration(duration))
			return true, duration
		case ipv4.ICMPTypeDestinationUnreachable:
			fmt.Printf("ICMP ping to %s failed: Destination Unreachable\n", hostStr)
			return false, duration
		case ipv4.ICMPTypeTimeExceeded:
			fmt.Printf("ICMP ping to %s failed: Time Exceeded\n", hostStr)
			return false, duration
		default:
			fmt.Printf("ICMP ping to %s unsuccessful (ICMP type: %v, code: %d)\n", hostStr, parsed_reply.Type, parsed_reply.Code)
			return false, duration
		}
	}
}

// print_certificate establishes a TLS connection to print certificate information
func print_certificate(host string, ip *net.IPAddr, port string, timeout time.Duration) (bool, time.Duration) {
	config := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	var hostStr string
	if host == ip.String() {
		hostStr = ip.String()
	} else {
		hostStr = fmt.Sprintf("%s (%s)", host, ip.String())
	}

	dialer := &net.Dialer{Timeout: timeout}
	start := time.Now()
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort(ip.String(), port), config)
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("TLS connection to %s:%s failed: %v\n", hostStr, port, err)
		return false, duration
	}
	defer conn.Close()

	state := conn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		fmt.Printf("TLS connection to %s:%s successful, but no certificates presented.\n", hostStr, port)
		return false, duration
	}

	cert := state.PeerCertificates[0]
	fmt.Printf("TLS connection to %s:%s successful (RTT: %v)\n", hostStr, port, formatDuration(duration))
	cipherName := tls.CipherSuiteName(state.CipherSuite)
	if cipherName == "" {
		cipherName = fmt.Sprintf("0x%04X", state.CipherSuite)
	}
	fmt.Printf("Negotiated TLS:  %s with cipher %s\n\n", tlsVersionString(state.Version), cipherName)
	fmt.Printf("Certificate Details:\n")
	fmt.Printf("  Subject (CN): %s\n", cert.Subject.CommonName)
	if len(cert.Subject.Organization) > 0 {
		fmt.Printf("  Organization: %s\n", cert.Subject.Organization[0])
	}
	var issuerStr string
	if len(cert.Issuer.Organization) > 0 {
		if cert.Issuer.CommonName != "" {
			issuerStr = fmt.Sprintf("%s (%s)", cert.Issuer.Organization[0], cert.Issuer.CommonName)
		} else {
			issuerStr = cert.Issuer.Organization[0]
		}
	} else {
		issuerStr = cert.Issuer.CommonName
	}
	fmt.Printf("  Issuer:       %s\n", issuerStr)
	fmt.Printf("  Valid From:   %s\n", cert.NotBefore.Format("2006-01-02 15:04:05 UTC"))
	fmt.Printf("  Valid Until:  %s\n", cert.NotAfter.Format("2006-01-02 15:04:05 UTC"))

	// Check expiration status
	now := time.Now()
	if now.Before(cert.NotBefore) {
		fmt.Printf("  Status:       Not yet valid (starts in %v)\n", cert.NotBefore.Sub(now).Round(time.Hour))
	} else if now.After(cert.NotAfter) {
		fmt.Printf("  Status:       Expired (expired %v ago)\n", now.Sub(cert.NotAfter).Round(time.Hour))
	} else {
		days := int(cert.NotAfter.Sub(now).Hours() / 24)
		if days > 0 {
			fmt.Printf("  Status:       Valid (expires in %d days)\n", days)
		} else {
			fmt.Printf("  Status:       Valid (expires in %v)\n", cert.NotAfter.Sub(now).Round(time.Minute))
		}
	}

	// Verification check
	var intermediates *x509.CertPool
	if len(state.PeerCertificates) > 1 {
		intermediates = x509.NewCertPool()
		for _, c := range state.PeerCertificates[1:] {
			intermediates.AddCert(c)
		}
	}

	opts := x509.VerifyOptions{
		DNSName:       host,
		Intermediates: intermediates,
	}
	_, verifyErr := cert.Verify(opts)
	if verifyErr != nil {
		fmt.Printf("  Verification: Untrusted/Invalid (%v)\n", verifyErr)
	} else {
		fmt.Printf("  Verification: Trusted\n")
	}

	if len(cert.DNSNames) > 0 {
		if len(cert.DNSNames) > 5 {
			fmt.Printf("  SANs:         %v... (and %d more)\n", cert.DNSNames[:5], len(cert.DNSNames)-5)
		} else {
			fmt.Printf("  SANs:         %v\n", cert.DNSNames)
		}
	}

	return true, duration
}

// printSummary prints end-of-run ping summary metrics
func printSummary(host string, total, success int, rtts []time.Duration) {
	fmt.Printf("\n--- %s netprobe statistics ---\n", host)
	loss := float64(total-success) / float64(total) * 100
	fmt.Printf("%d probes sent, %d successful, %.1f%% loss\n", total, success, loss)
	if len(rtts) > 0 {
		min := rtts[0]
		max := rtts[0]
		var totalRTT time.Duration
		for _, rtt := range rtts {
			if rtt < min {
				min = rtt
			}
			if rtt > max {
				max = rtt
			}
			totalRTT += rtt
		}
		avg := totalRTT / time.Duration(len(rtts))
		fmt.Printf("Round-trip min/avg/max = %s/%s/%s\n", formatDuration(min), formatDuration(avg), formatDuration(max))
	}
}

// tlsVersionString converts TLS protocol version uint16 to its string representation
func tlsVersionString(v uint16) string {
	switch v {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("TLS 0x%04X", v)
	}
}

// run_traceroute executes an ICMP-based traceroute to the destination
func run_traceroute(host string, ip *net.IPAddr, timeout time.Duration) {
	isIPv6 := ip.IP.To4() == nil
	network := "udp4"
	proto := 1 // ICMPv4 protocol number
	msgType := icmp.Type(ipv4.ICMPTypeEcho)
	listenAddr := "0.0.0.0"

	if isIPv6 {
		network = "udp6"
		proto = 58 // ICMPv6 protocol number
		msgType = ipv6.ICMPTypeEchoRequest
		listenAddr = "::"
	}

	var conn *icmp.PacketConn
	var err error
	var raw = true

	// Attempt raw socket first for full traceroute path
	if isIPv6 {
		conn, err = icmp.ListenPacket("ip6:ipv6-icmp", listenAddr)
		if err != nil {
			raw = false
			conn, err = icmp.ListenPacket(network, listenAddr)
		}
	} else {
		conn, err = icmp.ListenPacket("ip4:icmp", listenAddr)
		if err != nil {
			raw = false
			conn, err = icmp.ListenPacket(network, listenAddr)
		}
	}

	if err != nil {
		fmt.Printf("Traceroute error: failed to listen on socket: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	if !raw {
		fmt.Println("Note: Running in unprivileged mode. Intermediate hops may time out.")
		fmt.Println("      Run with sudo / root permissions to view all intermediate routers.")
	}

	var hostStr string
	if host == ip.String() {
		hostStr = ip.String()
	} else {
		hostStr = fmt.Sprintf("%s (%s)", host, ip.String())
	}

	fmt.Printf("\nTraceroute to %s, 30 hops max:\n", hostStr)

	maxHops := 30
	for hop := 1; hop <= maxHops; hop++ {
		// Set TTL/Hop Limit
		if isIPv6 {
			ipv6Conn := conn.IPv6PacketConn()
			if ipv6Conn != nil {
				ipv6Conn.SetHopLimit(hop)
			}
		} else {
			ipv4Conn := conn.IPv4PacketConn()
			if ipv4Conn != nil {
				ipv4Conn.SetTTL(hop)
			}
		}

		msg := icmp.Message{
			Type: msgType,
			Code: 0,
			Body: &icmp.Echo{
				ID:   os.Getpid() & 0xffff,
				Seq:  hop,
				Data: []byte("ping"),
			},
		}
		msg_bytes, err := msg.Marshal(nil)
		if err != nil {
			fmt.Printf(" %2d  Error marshalling packet: %v\n", hop, err)
			continue
		}

		start := time.Now()
		dst := &net.UDPAddr{IP: ip.IP}

		if _, err := conn.WriteTo(msg_bytes, dst); err != nil {
			fmt.Printf(" %2d  Error sending packet: %v\n", hop, err)
			continue
		}

		err = conn.SetReadDeadline(time.Now().Add(timeout))
		if err != nil {
			fmt.Printf(" %2d  Error setting timeout: %v\n", hop, err)
			continue
		}

		reply := make([]byte, 1500)
		n, peer, err := conn.ReadFrom(reply)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf(" %2d  *\n", hop)
			continue
		}

		parsed_reply, err := icmp.ParseMessage(proto, reply[:n])
		if err != nil {
			fmt.Printf(" %2d  Error parsing reply from %v\n", hop, peer)
			continue
		}

		// Parse peer host address
		peerIP := peer.String()
		if h, _, err := net.SplitHostPort(peerIP); err == nil {
			peerIP = h
		}

		if isIPv6 {
			switch parsed_reply.Type {
			case ipv6.ICMPTypeTimeExceeded:
				fmt.Printf(" %2d  %s (%s)\n", hop, peerIP, formatDuration(duration))
			case ipv6.ICMPTypeEchoReply:
				fmt.Printf(" %2d  %s (%s) [Reached Destination]\n", hop, peerIP, formatDuration(duration))
				return
			default:
				fmt.Printf(" %2d  %s (Type: %v)\n", hop, peerIP, parsed_reply.Type)
			}
		} else {
			switch parsed_reply.Type {
			case ipv4.ICMPTypeTimeExceeded:
				fmt.Printf(" %2d  %s (%s)\n", hop, peerIP, formatDuration(duration))
			case ipv4.ICMPTypeEchoReply:
				fmt.Printf(" %2d  %s (%s) [Reached Destination]\n", hop, peerIP, formatDuration(duration))
				return
			default:
				fmt.Printf(" %2d  %s (Type: %v)\n", hop, peerIP, parsed_reply.Type)
			}
		}
	}
}
