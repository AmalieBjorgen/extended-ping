package main

import (
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func main() {
	args := os.Args
	if len(args) != 2 {
		fmt.Println("Usage: eping <host>")
		os.Exit(1)
	}

	host := args[1]
	ip, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		fmt.Println("Cannot resolve IP address to host.")
		os.Exit(1)
	}
	var timeout time.Duration = time.Second * 3

	common_ports := [...]string{"20", "21", "22", "23", "25", "53", "80", "110", "143", "443", "3389", "8080"}

	icmp_ping(ip)

	for _, port := range common_ports {
		tcp_ping(ip, port, timeout)
	}

	for _, port := range common_ports {
		udp_ping(ip, port, timeout)
	}

}

func tcp_ping(host *net.IPAddr, port string, timeout time.Duration) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", fmt.Sprintf("%v:%v", host, port))
	if err != nil {
		fmt.Printf("TCP ping to %s:%s unsuccessful.\n", host, port)
		return
	}
	defer conn.Close()

	fmt.Printf("TCP ping to %s:%s successful.\n", host, port)
}

func udp_ping(host *net.IPAddr, port string, timeout time.Duration) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("udp", fmt.Sprintf("%v:%v", host, port))
	if err != nil {
		fmt.Printf("UDP ping to %s:%s unsuccessful.\n", host, port)
		return
	}
	defer conn.Close()

	fmt.Printf("UDP ping to %s:%s successful.\n", host, port)
}

func icmp_ping(host *net.IPAddr) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Println("Error2: ", err)
		return
	}

	defer conn.Close()

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte(""),
		},
	}
	msg_bytes, err := msg.Marshal(nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	if _, err := conn.WriteTo(msg_bytes, host); err != nil {
		fmt.Println(err)
		panic(err)
	}

	err = conn.SetReadDeadline(time.Now().Add(time.Second * 1))
	if err != nil {
		fmt.Println(err)
		return
	}
	reply := make([]byte, 644)
	n, _, err := conn.ReadFrom(reply)

	if err != nil {
		fmt.Println(err)
		return
	}

	parsed_reply, err := icmp.ParseMessage(1, reply[:n])

	if err != nil {
		fmt.Println(err)
		return
	}

	switch parsed_reply.Code {
	case 0:
		// Got a reply so we can save this
		fmt.Printf("ICMP ping to %s successful.\n", host)
	case 3:
		fmt.Printf("Host %s is unreachable\n", host)
		// Given that we don't expect google to be unreachable, we can assume that our network is down
	case 11:
		// Time Exceeded so we can assume our network is slow
		fmt.Printf("Host %s is slow\n", host)
	default:
		// We don't know what this is so we can assume it's unreachable
		fmt.Printf("Host %s is unreachable\n", host)
	}
}
