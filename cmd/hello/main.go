package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/transport"
)

func main() {
	device := flag.String("dev", "", "Serial device, such as /dev/ttyUSB0")
	flag.Parse()

	log.Println("Waiting for KV4P HELLO.")
	log.Println("Press the KV4P EN/RESET button once.")

	for {
		portName := *device

		if portName == "" {
			found, err := transport.FindKV4P()
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			portName = found
		}

		log.Printf("Connecting to %s", portName)

		c, err := client.Connect(portName)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		hello, err := waitForHello(c)
		_ = c.Close()

		if err != nil {
			log.Printf("Connection lost: %v; reconnecting...", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		fmt.Println("========== KV4P HELLO ==========")
		fmt.Printf("Protocol Version : %d\n", hello.Version)
		fmt.Printf("Radio Status     : %c\n", hello.RadioStatus)
		fmt.Printf("Window Size      : %d\n", hello.WindowSize)
		fmt.Printf("RF Module        : %d\n", hello.RFModuleType)
		fmt.Printf("RX Range         : %.3f - %.3f MHz\n",
			hello.MinFrequencyMHz,
			hello.MaxFrequencyMHz,
		)
		fmt.Printf("Features         : 0x%02X\n", hello.Features)
		fmt.Println("================================")
		return
	}
}

func waitForHello(c *client.Client) (protocol.Hello, error) {
	for {
		frame, err := c.ReadFrame()
		if err != nil {
			return protocol.Hello{}, err
		}

		vendor, err := protocol.DecodeVendorFrame(frame)
		if err != nil {
			continue
		}

		log.Printf(
			"Received KV4P command 0x%02X with %d payload bytes",
			vendor.Command,
			len(vendor.Payload),
		)

		if vendor.Command != protocol.CommandHello {
			continue
		}

		return protocol.ParseHello(vendor.Payload)
	}
}
