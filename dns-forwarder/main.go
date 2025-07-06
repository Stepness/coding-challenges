package main

import (
	"dns-forwarder/dns"
	"fmt"
	"net"
)

func main() {
	fmt.Println("DNS started")

	udpAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:8053")
	if err != nil {
		fmt.Println("Failed to resolve UDP address:", err)
		return
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		fmt.Println("Failed to bind to address:", err)
		return
	}

	defer udpConn.Close()

	buf := make([]byte, 512)

	for {
		size, addr, err := udpConn.ReadFromUDP(buf)

		if err != nil {
			fmt.Println("Failed to receive UDP message:", err)
			return
		}

		dnsRequest := dns.ReadDNSMessage(buf)
		fmt.Println("Header -", "ID:", dnsRequest.Header.ID)
		fmt.Println("Question -", "Name:", string(dnsRequest.Question.QNAME), "Type:", dnsRequest.Question.QTYPE, "Class:", dnsRequest.Question.QCLASS)

		googleAddr, err := net.ResolveUDPAddr("udp", "8.8.8.8:53")
		if err != nil {
			fmt.Println("Failed to resolve Google DNS:", err)
			continue
		}
		connToGoogle, err := net.DialUDP("udp", nil, googleAddr)

		if err != nil {
			fmt.Println("Failed to connect to Google DNS:", err)
			continue
		}

		_, err = connToGoogle.Write(buf[:size])
		if err != nil {
			fmt.Println("Failed to send to Google DNS:", err)
			continue
		}

		responseBuf := make([]byte, 512)
		n, err := connToGoogle.Read(responseBuf)
		if err != nil {
			fmt.Println("Failed to read from Google DNS:", err)
			continue
		}

		connToGoogle.Close()

		_, err = udpConn.WriteToUDP(responseBuf[:n], addr)
		if err != nil {
			fmt.Println("Failed to send response to client:", err)
			continue
		}
	}
}
