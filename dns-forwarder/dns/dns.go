package dns

import "encoding/binary"

type DNS_Message struct {
	Header   DNS_Header
	Question DNS_Question
	Answer   DNS_Answer
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

func ReadDNSMessage(buf []byte) DNS_Message {
	var message DNS_Message

	message.Header = readHeader(buf)
	message.Question = readQuestion(buf[12:])
	message.Answer = readAnswer(buf[12+len(message.Question.QNAME)+4:])

	return message
}

func WriteDNSMessage(msg DNS_Message) []byte {

	arr := writeHeader(msg.Header)
	arr = append(arr, writeQuestion(msg.Question)...)
	arr = append(arr, writeAnswer(msg.Answer)...)

	return arr
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

func readAnswer(buf []byte) (answer DNS_Answer) {
	nameSize := 0

	if len(buf) == 0 {
		return
	}

	for buf[nameSize] != 0x00 {
		nameSize += 1 + int(buf[nameSize]) //Skip cycles of the labels
	}
	nameSize++ //Include 0x00
	answer.NAME = buf[:nameSize]
	answer.TYPE = binary.BigEndian.Uint16(buf[nameSize : nameSize+2])
	answer.CLASS = binary.BigEndian.Uint16(buf[nameSize+2 : nameSize+4])
	answer.TTL = binary.BigEndian.Uint32(buf[nameSize+4 : nameSize+8])
	answer.RDLENGHT = binary.BigEndian.Uint16(buf[nameSize+8 : nameSize+10])
	answer.RDATA = buf[nameSize+10 : nameSize+10+int(answer.RDLENGHT)]
	return
}

func writeHeader(header DNS_Header) []byte {
	arr := make([]byte, 2)

	binary.BigEndian.PutUint16(arr, header.ID)

	var flags uint16

	header.QR = 1

	flags |= (uint16(header.QR) << 15)
	flags |= (uint16(header.OPCODE) << 11)
	flags |= (uint16(header.AA) << 10)
	flags |= (uint16(header.TC) << 9)
	flags |= (uint16(header.RD) << 8)
	flags |= (uint16(header.RA) << 7)
	flags |= (uint16(header.Z) << 4)
	flags |= uint16(header.RCODE)

	binary.BigEndian.AppendUint16(arr, flags)
	binary.BigEndian.AppendUint16(arr, header.QDCOUNT)
	binary.BigEndian.AppendUint16(arr, header.ANCOUNT)
	binary.BigEndian.AppendUint16(arr, header.NSCOUNT)
	binary.BigEndian.AppendUint16(arr, header.ARCOUNT)
	return arr
}

func writeQuestion(question DNS_Question) []byte {
	var arr []byte

	arr = append(arr, question.QNAME...)
	arr = binary.BigEndian.AppendUint16(arr, question.QTYPE)
	arr = binary.BigEndian.AppendUint16(arr, question.QCLASS)
	return arr
}

func writeAnswer(answer DNS_Answer) []byte {
	var arr []byte

	arr = append(arr, answer.NAME...)
	arr = binary.BigEndian.AppendUint16(arr, answer.TYPE)
	arr = binary.BigEndian.AppendUint16(arr, answer.CLASS)
	arr = binary.BigEndian.AppendUint32(arr, answer.TTL)
	arr = binary.BigEndian.AppendUint16(arr, answer.RDLENGHT)
	arr = append(arr, answer.RDATA...)

	return arr
}
