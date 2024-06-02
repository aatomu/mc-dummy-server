package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

const segmentBit int = 0x7f
const continueBit int = 0x80

var (
	port = flag.Int("port", 25565, "Listen Port")
)

func main() {
	flag.Parse()

	server, err := net.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	for {
		conn, err := server.Accept()
		if err != nil {
			break
		}

		reader := bufio.NewReader(conn)

		var isLogin bool = false
		var accessDomain = ""
		for {
			fmt.Println("=======================================")
			length, _ := readVarInt(reader)
			if length == 0 {
				log.Println("Close Connection")
				break
			}

			data := make([]byte, length)
			reader.Read(data)

			buf := bytes.NewReader(data)
			packetId, packetIdLen := readVarInt(buf)

			dataLen := length - packetIdLen
			if dataLen == 0 {
				continue
			}
			fmt.Println("Data", data[1:])
			// Status
			if packetId == 0 && dataLen > 0 {
				if isLogin {
					nameLen, _ := readVarInt(buf)
					name := make([]byte, nameLen)
					buf.Read(name)
					log.Printf("Login: %s", name)

					data := fmt.Sprintf(`[{"text":"Hi! %s\n","color":"gold"},{"text":"Access IP: %s\n","color":"gray"},{"text":"Address: %s\n","color":"gray"},{"text":"You can't log in to this server.","color":"green"}]`, name, conn.RemoteAddr().String(), accessDomain)
					dataLen := writeVarInt(len(data))
					conn.Write(newResponse(0x00, append(dataLen, data...)))
					continue
				}

				protocolVer, _ := readVarInt(buf)
				addressLen, _ := readVarInt(buf)
				address := make([]byte, addressLen)
				buf.Read(address)
				portData := make([]byte, 2)
				buf.Read(portData)
				port := binary.BigEndian.Uint16(portData)
				nextState, _ := readVarInt(buf)
				log.Printf("Status Protocol: %d, Address: %s:%d Next: %d", protocolVer, address, port, nextState)
				accessDomain = fmt.Sprintf("%s:%d", address, port)
				if nextState == 2 {
					isLogin = true
					continue
				}

				data := fmt.Sprintf(`{"version":{"name":"Why looking here?","protocol":%d},"players":{"max":0,"online":-21263,"sample":[{"name":"aatomu","id":"c52fafa6-e223-4bdd-b884-b39f641a4cf4"}]},"description":[{"text":"SERVER","color":"gold","bold":true},{"text":" "},{"text":"IS","color":"red","bold":true},{"text":" "},{"text":"SERVER\n","color":"blue","bold":true},{"text":"Time: %s"}]}`, protocolVer, time.Now().Format("2006-01-02T15:04:05.00 UTC-07:00"))
				dataLen := writeVarInt(len(data))
				conn.Write(newResponse(0, append(dataLen, data...)))
				continue
			}
			// Status Ping
			if packetId == 1 && dataLen > 0 {
				payload := make([]byte, dataLen)
				buf.Read(payload)
				log.Printf("Status Ping: %d", payload)
				conn.Write(newResponse(1, payload))
			}
		}
	}
}

func readVarInt(reader io.Reader) (value int, length int) {
	buf := make([]byte, 1)
	position := 0
	for {
		length++
		reader.Read(buf)
		current := buf[0]
		value = value | (int(current) & segmentBit << position)
		if current&byte(continueBit) == 0 {
			break
		}
		position += 7
		if position >= 32 {
			break
		}
	}
	return
}

func writeVarInt(value int) []byte {
	buf := []byte{}

	for {
		if value < continueBit {
			buf = append(buf, uint8(value))
			return buf
		}

		buf = append(buf, uint8(value&segmentBit|continueBit))
		value = value >> 7
	}
}

func newResponse(packetId int, data []byte) []byte {
	response := []byte{}
	id := writeVarInt(packetId)
	idLen := writeVarInt(len(id))
	response = append(response, writeVarInt(len(idLen)+len(data))...)
	response = append(response, id...)
	response = append(response, data...)
	return response
}
