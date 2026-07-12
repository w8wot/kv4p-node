package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"go.bug.st/serial"
)

func main() {
	mode := &serial.Mode{
		BaudRate: 115200,
		DataBits: 8,
		StopBits: serial.OneStopBit,
		Parity:   serial.NoParity,
	}

	port, err := serial.Open("/dev/ttyUSB0", mode)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	_ = port.SetReadTimeout(500 * time.Millisecond)

	log.Println("Resetting KV4P...")

	_ = port.SetDTR(false)
	_ = port.SetRTS(true)
	time.Sleep(100 * time.Millisecond)

	_ = port.SetDTR(true)
	_ = port.SetRTS(false)
	time.Sleep(100 * time.Millisecond)

	_ = port.SetDTR(false)
	_ = port.SetRTS(true)

	deadline := time.Now().Add(8 * time.Second)
	buf := make([]byte, 1024)

	for time.Now().Before(deadline) {
		n, err := port.Read(buf)
		if err != nil {
			log.Fatal(err)
		}

		if n > 0 {
			fmt.Print(hex.Dump(buf[:n]))
		}
	}
}
