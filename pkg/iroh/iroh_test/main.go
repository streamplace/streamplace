package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"encoding/hex"

	iroh "stream.place/streamplace/pkg/iroh/generated/iroh_streamplace"
	_ "stream.place/streamplace/pkg/streamplacedeps"
)

func panicIfErr(err error) {
	if err != nil {
		panic(err)
	}
}

func GetOrCreateIrohSecret() ([]byte, error) {
    hexStr := os.Getenv("IROH_SECRET")

    if hexStr != "" {
        // User provided one — it must be valid
        decoded, err := hex.DecodeString(hexStr)
        if err != nil {
            return nil, fmt.Errorf("IROH_SECRET: invalid hex string: %w", err)
        }
        if len(decoded) != 32 {
            return nil, fmt.Errorf("IROH_SECRET: must be 32 bytes when decoded (64 hex chars), got %d", len(decoded))
        }
        return decoded, nil
    }

    // No env var → generate random secret
    secret := make([]byte, 32)
    if _, err := rand.Read(secret); err != nil {
        return nil, fmt.Errorf("failed to generate random secret: %w", err)
    }

    // Super helpful during local dev / first run
    fmt.Fprintf(os.Stderr, "No IROH_SECRET set — generated new random secret:\n")
    fmt.Fprintf(os.Stderr, "%s\n", hex.EncodeToString(secret))

    return secret, nil
}

func main() {
	tickets := os.Args[1:]

	secret, err := GetOrCreateIrohSecret()
	panicIfErr(err)

	config := iroh.SocketConfig {
		Secret:             secret,
		Alpn: 						  []byte("iroh-streamplace/0.1.0"),
	}
	fmt.Println("Creating iroh socket")
	socket, err := iroh.NewSocket(config)
	panicIfErr(err)

	fmt.Println("Node created:", socket)
	if len(tickets) > 0 {
		fmt.Println("Ticket:", tickets[0])
		addr, err := iroh.NodeAddrFromTicket(tickets[0])
		panicIfErr(err)
		fmt.Println("Connecting to addr:", addr)
		stream, err := socket.Connect(addr)
		panicIfErr(err)
		fmt.Println("Connected to stream:", stream)
		err2 := stream.WriteAll([]byte("Hello from client!\n"))
		panicIfErr(err2)
		err3 := stream.CloseWrite()
		panicIfErr(err3)
		data, err4 := stream.Read(1024)
		panicIfErr(err4)
		fmt.Println("Received data:", string(data))
		stream.Close()
		socket.Close()
	} else {
		fmt.Println("Waiting for socket to become online")
		socket.Online()
		fmt.Println("Awaiting incoming connections")
		fmt.Println("Ticket: ", socket.Ticket())
		stream, err := socket.Accept()
		panicIfErr(err)
		fmt.Println("Got incoming connection", stream)
		data, err := stream.Read(1024)
		panicIfErr(err)
		fmt.Println("Received data:", string(data))
		err2 := stream.WriteAll([]byte("Hello from server!\n"))
		panicIfErr(err2)
		err3 := stream.CloseWrite()
		panicIfErr(err3)
		stream.Closed()
	}
}
