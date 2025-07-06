package main

import (
	"dns-forwarder/dns"
	"testing"
)

func TestReadDNSRequest(t *testing.T) {

	dnsQuery := []byte{
		0x1a, 0x2b, // ID
		0x01, 0x00, // Flags
		0x00, 0x01, // Question
		0x00, 0x00,
		0x00, 0x00,
		0x00, 0x00,
		0x03, 'w', 'w', 'w',
		0x06, 'g', 'o', 'o', 'g', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
		0x00, 0x01, // Type A
		0x00, 0x01, // Class IN
	}

	msg := dns.ReadDNSMessage(dnsQuery)

	if msg.Header.ID != 0x1a2b {
		t.Errorf("Expected ID 0xaabb, got 0x%x", msg.Header.ID)
	}

	if msg.Header.QDCOUNT != 1 {
		t.Errorf("Expected QDCOUNT 1, got %d", msg.Header.QDCOUNT)
	}

	wantQNAME := []byte{
		0x03, 'w', 'w', 'w',
		0x06, 'g', 'o', 'o', 'g', 'l', 'e',
		0x03, 'c', 'o', 'm',
		0x00,
	}
	if string(msg.Question.QNAME) != string(wantQNAME) {
		t.Errorf("QNAME mismatch: got %v, want %v", msg.Question.QNAME, wantQNAME)
	}

	if msg.Question.QTYPE != 1 {
		t.Errorf("Expected QTYPE 1 (A), got %d", msg.Question.QTYPE)
	}
	if msg.Question.QCLASS != 1 {
		t.Errorf("Expected QCLASS 1 (IN), got %d", msg.Question.QCLASS)
	}
}
