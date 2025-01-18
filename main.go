package main

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

const segmentBit int = 0x7f
const continueBit int = 0x80

type MinecraftServer struct {
	Version     ServerVersion    `json:"version"`
	Players     ServerPlayers    `json:"players"`
	Description []map[string]any `json:"description"`
	// 64x64px png image & base64encoded!!
	// Favicon:"data:image/png;base64,<data>"
	Favicon            string `json:"favicon"`
	EnforcesSecureChat bool   `json:"enforcesSecureChat"`
}

type ServerVersion struct {
	Name     string `json:"name"`
	Protocol int    `json:"protocol"`
}

type ServerPlayers struct {
	Max    int            `json:"max"`
	Online int            `json:"online"`
	Sample []ServerPlayer `json:"sample"`
}

type ServerPlayer struct {
	Name string `json:"name"`
	Id   string `json:"id"`
}

type LoginFailedReason []map[string]any

var (
	port = flag.Int("port", 25565, "Listen Port")
	id   = 0
)

func main() {
	flag.Parse()

	server, err := net.Listen("tcp4", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	log.Println(strings.Repeat("=", 20), "Listener booted", strings.Repeat("=", 20))
	for {
		conn, err := server.Accept()
		if err != nil {
			break
		}
		id++
		go NewSession(conn, id)
	}
}

func NewSession(conn net.Conn, id int) {
	reader := bufio.NewReader(conn)

	var isLogin bool = false
	var accessDomain = ""

	log.Printf("No%04d: State:\"New connection\" IP:%s", id, conn.RemoteAddr().String())
	for {
		length, _ := readVarInt(reader)
		if length == 0 {
			log.Printf("No%04d: State:\"Close session\"", id)
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

		// Status
		if packetId == 0 && dataLen > 0 {
			if isLogin {
				nameLen, _ := readVarInt(buf)
				name := make([]byte, nameLen)
				buf.Read(name)
				log.Printf("No%04d: State:\"Login\" MCID:\"%s\" AccessDomain:\"%s\" Raw:%v", id, name, accessDomain, data[1:])

				reason := LoginFailedReason{
					{"text": fmt.Sprintf("Hi! %s\n", name), "color": "gold"},
					{"text": fmt.Sprintf("Access IP: %s\n", conn.RemoteAddr().String()), "color": "gray"},
					{"text": fmt.Sprintf("Address: %s\n", accessDomain), "color": "gray"},
					{"text": "You can't login to this server.", "color": "green"},
				}
				data, _ := json.Marshal(reason)
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
			log.Printf("No%04d: State:\"Handshake\" Protocol:%d Address:\"%s:%d\" Next:%d Raw:%v", id, protocolVer, address, port, nextState, data[1:])
			accessDomain = fmt.Sprintf("%s:%d", address, port)
			if nextState == 2 {
				isLogin = true
				continue
			}

			status := MinecraftServer{
				Version: ServerVersion{
					Name:     "How to looking here?",
					Protocol: protocolVer,
				},
				Players: ServerPlayers{
					Max:    0,
					Online: -21263,
					Sample: []ServerPlayer{
						{
							Name: "aatomu",
							Id:   "c52fafa6-e223-4bdd-b884-b39f641a4cf4",
						},
					},
				},
				Description: []map[string]any{
					{"text": "SERVER", "color": "gold", "bold": true},
					{"text": " "},
					{"text": "IS", "color": "red", "bold": true},
					{"text": " "},
					{"text": "SERVER\n", "color": "blue", "bold": true},
					{"text": fmt.Sprintf("Time: %s", time.Now().Format("2006-01-02T15:04:05.00 UTC-07:00"))},
				},
			}

			func() {
				_, err := os.Stat("./icon.png")
				if err == nil {
					img, err := os.ReadFile("./icon.png")
					if err != nil {
						return
					}

					base := base64.StdEncoding.EncodeToString(img)
					status.Favicon = fmt.Sprintf("data:image/png;base64,%s", base)
				}
			}()
			data, _ := json.Marshal(status)
			dataLen := writeVarInt(len(data))
			conn.Write(newResponse(0, append(dataLen, data...)))
			continue
		}
		// Status Ping
		if packetId == 1 && dataLen > 0 {
			payload := make([]byte, dataLen)
			buf.Read(payload)
			log.Printf("No%04d: State:\"Ping\" Payload:%v Raw:%v", id, payload, data[1:])
			conn.Write(newResponse(1, payload))
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
	response = append(response, writeVarInt(len(id)+len(data))...)
	response = append(response, id...)
	response = append(response, data...)
	return response
}
