package main

import (
	"flag"
	"log"
	"time"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/transport"
)

func main() {
	freq := flag.Float64("freq", 146.520, "Receive frequency in MHz")
	squelch := flag.Int("squelch", 8, "Squelch level 0-8")
	flag.Parse()

	portName, err := transport.FindKV4P()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Opening %s without resetting", portName)

	c, err := client.Connect(portName)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	sequence := uint32(time.Now().Unix())

	state := protocol.HostDesiredState{
		Sequence: sequence,
		MemoryID: -1,
		Flags: protocol.HostStateRadioConfigValid |
			protocol.HostStateRXAudioOpen |
			protocol.HostStateRSSIEnabled |
			protocol.HostStateFilterHigh |
			protocol.HostStateFilterLow |
			protocol.HostStateStatusReports,
		Bandwidth:      1,
		TXFrequencyMHz: float32(*freq),
		RXFrequencyMHz: float32(*freq),
		Squelch:        byte(*squelch),
	}

	frame, err := protocol.EncodeDesiredStateFrame(state)
	if err != nil {
		log.Fatal(err)
	}

	if err := c.Write(frame); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"State requested: sequence=%d frequency=%.3f squelch=%d",
		sequence,
		*freq,
		*squelch,
	)

	var (
		haveLast bool
		last     protocol.DeviceState
	)

	for {
		kissFrame, err := c.ReadFrame()
		if err != nil {
			log.Fatal(err)
		}

		vendor, err := protocol.DecodeVendorFrame(kissFrame)
		if err != nil || vendor.Command != protocol.CommandDeviceState {
			continue
		}

		current, err := protocol.ParseDeviceState(vendor.Payload)
		if err != nil {
			log.Printf("Bad state: %v", err)
			continue
		}

		if haveLast &&
			current.Flags == last.Flags &&
			current.LatestRSSI == last.LatestRSSI &&
			current.LastError == last.LastError {
			continue
		}

		haveLast = true
		last = current

		log.Printf(
			"seq=%d flags=0x%04X squelched=%t tx=%t rssi=%d error=%d",
			current.AppliedSequence,
			current.Flags,
			current.HasFlag(protocol.DeviceStateSquelched),
			current.HasFlag(protocol.DeviceStateTXActive),
			current.LatestRSSI,
			current.LastError,
		)
	}
}
