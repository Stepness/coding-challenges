package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

type DNS_Message struct {
	header   DNS_Header
	question DNS_Question
	answer   DNS_Answer
}

type DNS_Header struct {
	ID      uint16
	QR      byte
	OPCODE  byte
	AA      byte
	TC      byte
	RD      byte
	RA      byte
	Z       byte
	RCODE   byte
	QDCOUNT uint16
	ANCOUNT uint16
	NSCOUNT uint16
	ARCOUNT uint16
}

type DNS_Question struct {
	QNAME  []byte
	QTYPE  uint16
	QCLASS uint16
}

type DNS_Answer struct {
	NAME     []byte
	TYPE     uint16
	CLASS    uint16
	TTL      uint32
	RDLENGHT uint16
	RDATA    []byte
}

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
		fmt.Println("Received size", size, "addr", addr)

		receivedData := string(buf[:size])
		fmt.Println("Received:", receivedData)

		dnsMessage := readDNSRequest(buf)
		fmt.Print(dnsMessage)

		udpConn.WriteToUDP(buf, addr)
	}
}

func readDNSRequest(buf []byte) DNS_Message {
	var message DNS_Message

	message.header = readHeader(buf)
	message.question = readQuestion(buf[12:])

	return message
}

func readHeader(buf []byte) DNS_Header {
	var header DNS_Header

	header.ID = binary.BigEndian.Uint16(buf[0:2])
	flags := binary.BigEndian.Uint16(buf[2:4])

	header.QR = byte((flags >> 15) & 0x1)
	header.OPCODE = byte((flags >> 11) & 0xF)
	header.AA = byte((flags >> 10) & 0x1)
	header.TC = byte((flags >> 9) & 0x1)
	header.RD = byte((flags >> 8) & 0x1)
	header.RA = byte((flags >> 7) & 0x1)
	header.Z = byte((flags >> 4) & 0x7)
	header.RCODE = byte((flags) & 0xF)

	header.QDCOUNT = binary.BigEndian.Uint16(buf[4:6])
	header.ANCOUNT = binary.BigEndian.Uint16(buf[6:8])
	header.NSCOUNT = binary.BigEndian.Uint16(buf[8:10])
	header.ARCOUNT = binary.BigEndian.Uint16(buf[10:12])

	return header
}

func readQuestion(buf []byte) (question DNS_Question) {
	qnameSize := 0
	for buf[qnameSize] != 0x00 {
		qnameSize += 1 + int(buf[qnameSize]) //Skip cycles of the labels
	}

	qnameSize++ //Include 0x00

	question.QNAME = buf[:qnameSize]
	question.QTYPE = binary.BigEndian.Uint16(buf[qnameSize : qnameSize+2])
	question.QCLASS = binary.BigEndian.Uint16(buf[qnameSize+2 : qnameSize+4])
	return
}
