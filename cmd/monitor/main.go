package main

import (
	"flag"
	"log"

	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/radio"
)

func main() {
	freq := flag.Float64("freq", 146.520, "Receive frequency in MHz")
	squelch := flag.Int("squelch", 3, "Squelch level 0-8")
	flag.Parse()

	log.Println("Start the monitor, then press the KV4P EN/RESET button once.")

	r, err := radio.Connect("")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	if err := r.ConfigureReceive(float32(*freq), byte(*squelch)); err != nil {
		log.Fatal(err)
	}

	log.Printf("Monitoring %.3f MHz", *freq)

	audioPackets := 0

	for {
		vendor, err := r.ReadVendorFrame()
		if err != nil {
			log.Fatal(err)
		}

		switch vendor.Command {
		case protocol.CommandRxAudio:
			audioPackets++
			log.Printf(
				"RX audio packet %d: %d bytes",
				audioPackets,
				len(vendor.Payload),
			)

		case protocol.CommandDeviceState:
			state, err := protocol.ParseDeviceState(vendor.Payload)
			if err != nil {
				log.Printf("Invalid device state: %v", err)
				continue
			}

			log.Printf(
				"State: squelched=%t TX=%t RSSI=%d error=%d",
				state.HasFlag(protocol.DeviceStateSquelched),
				state.HasFlag(protocol.DeviceStateTXActive),
				state.LatestRSSI,
				state.LastError,
			)
		}
	}
}
