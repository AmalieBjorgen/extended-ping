package main

import (
	"bufio"
	"fmt"
	"net"
	"time"
)

func main() {
	timeout := 15
	host := "google.com"
	port := "80"

	tcp_ping(host, port, time.Duration(timeout)*time.Second)
	udp_ping(host, port, time.Duration(timeout)*time.Second)

}

func tcp_ping(host string, port string, timeout time.Duration) {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%v:%v", host, port), timeout)
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
	conn, err := net.DialTimeout("udp", fmt.Sprintf("%v:%v", host, port), timeout)
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
