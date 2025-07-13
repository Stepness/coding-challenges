package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
)

type HTTP_Request struct {
	Verb        string
	Resource    string
	HTTPVersion string
}

type HTTP_Response struct {
	StatusCode string
	Content    []byte
}

func main() {
	fmt.Println("Web server started")
	listener, err := net.Listen("tcp", ":80")

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	defer listener.Close()

	for {
		conn, err := listener.Accept()

		if err != nil {
			fmt.Println(err.Error())
			return
		}

		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	fmt.Println("Serving", conn.RemoteAddr().String())
	buf := make([]byte, 1024)
	conn.Read(buf)

	request := readRequest(buf)

	fmt.Println(request)

	response := writeResponse(request)

	conn.Write(response)
}

func readRequest(buf []byte) HTTP_Request {
	var request HTTP_Request

	lines := strings.SplitAfter(string(buf), "\r\n")

	startLine := strings.Fields(lines[0])

	if len(startLine) < 3 {
		fmt.Println("invalid HTTP request line:", string(buf), "\nContains less than 3 values")
		return request
	}

	request.Verb = startLine[0]
	request.Resource = filepath.Clean(startLine[1])
	request.HTTPVersion = startLine[2]

	return request
}

func writeResponse(req HTTP_Request) []byte {

	fileLocation := "./www/"
	var httpResponse HTTP_Response

	if req.Resource == "/" {
		fileLocation = filepath.Join(fileLocation, "index.html")
	} else {
		fileLocation = filepath.Join(fileLocation, req.Resource)
	}

	if fileExists(fileLocation) {
		httpResponse.StatusCode = "200 OK"

		file, err := os.Open(fileLocation)

		if err != nil {
			fmt.Println(err)
		}

		httpResponse.Content, err = io.ReadAll(file)
		if err != nil {
			fmt.Println(err)
		}

		file.Close()
	} else {
		httpResponse.StatusCode = "400 Not Found"
	}

	response := []byte(fmt.Sprintf("%s %s\r\nRequested path: %s\r\n", req.HTTPVersion, httpResponse.StatusCode, req.Resource))
	response = append(response, []byte("\r\n")...)
	response = append(response, httpResponse.Content...)
	response = append(response, []byte("\r\n")...)

	return response
}

func fileExists(filePath string) bool {
	_, error := os.Stat(filePath)
	return !errors.Is(error, os.ErrNotExist)
}
