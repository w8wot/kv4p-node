package main

import (
	"log"

	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/radio"
)

func main() {
	log.Println("Waiting for KV4P HELLO. Press EN/RESET once.")

	r, err := radio.Connect("")
	if err != nil {
		log.Fatal(err)
	}
	defer r.Close()

	if err := r.ConfigureReceive(146.520, 3); err != nil {
		log.Fatal(err)
	}

	log.Println("Monitoring... Press EN/RESET once.")

	for {
		frame, err := r.ReadVendorFrame()
		if err != nil {
			log.Fatal(err)
		}

		if frame.Command != protocol.CommandDeviceState {
			continue
		}

		state, err := protocol.ParseDeviceState(frame.Payload)
		if err != nil {
			continue
		}

		log.Printf(
			"Flags=%04X  RSSI=%d  Squelched=%v  TX=%v",
			state.Flags,
			state.LatestRSSI,
			state.HasFlag(protocol.DeviceStateSquelched),
			state.HasFlag(protocol.DeviceStateTXActive),
		)
	}
}
