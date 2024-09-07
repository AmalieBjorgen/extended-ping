package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func main() {
	var timeout time.Duration = time.Second * 3
	host := "google.com"
	port := "80"

	tcp_ping(host, port, timeout)
	icmp_ping(host)
	//udp_ping(host, port, timeout)

}

func tcp_ping(host string, port string, timeout time.Duration) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("tcp", fmt.Sprintf("%v:%v", host, port))
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("TCP ping successful\n", status)
}

func udp_ping(host string, port string, timeout time.Duration) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.Dial("udp", fmt.Sprintf("%v:%v", host, port))
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	fmt.Println("UDP ping successful\n", status)
}

func icmp_ping(host string) {
	ip, err := net.ResolveIPAddr("ip4", host)
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		fmt.Println("Error: ", err)
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
		fmt.Println("Error: ", err)
		return
	}
	if _, err := conn.WriteTo(msg_bytes, &net.UDPAddr{IP: ip.IP}); err != nil {
		fmt.Println("Error: ", err)
		panic(err)
	}

	err = conn.SetReadDeadline(time.Now().Add(time.Second * 1))
	if err != nil {
		fmt.Println("Error: ", err)
		return
	}
	reply := make([]byte, 1500)
	n, _, err := conn.ReadFrom(reply)

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	parsed_reply, err := icmp.ParseMessage(1, reply[:n])

	if err != nil {
		fmt.Println("Error: ", err)
		return
	}

	switch parsed_reply.Code {
	case 0:
		// Got a reply so we can save this
		fmt.Printf("Got Reply from %s\n", host)
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
