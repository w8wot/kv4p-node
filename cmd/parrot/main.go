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

const (
	frameDuration  = 40 * time.Millisecond
	txFramePace    = 38 * time.Millisecond
	preTXDelay     = 500 * time.Millisecond
	keyupDelay     = 250 * time.Millisecond
	postAudioDelay = 150 * time.Millisecond
	postTXCooldown = 1 * time.Second

	startRSSI = 80
	stopRSSI  = 60
	stopCount = 3

	maxPackets = 750 // 30 seconds
	minPackets = 3
)

func main() {
	freq := flag.Float64("freq", 146.520, "Simplex frequency in MHz")
	squelch := flag.Int("squelch", 3, "Radio squelch level 0-8")
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

	desired := protocol.HostDesiredState{
		Sequence: sequence,
		MemoryID: -1,
		Flags: protocol.HostStateRadioConfigValid |
			protocol.HostStateRXAudioOpen |
			protocol.HostStateRSSIEnabled |
			protocol.HostStateFilterHigh |
			protocol.HostStateFilterLow |
			protocol.HostStateTXAllowed |
			protocol.HostStateStatusReports,
		Bandwidth:      1,
		TXFrequencyMHz: float32(*freq),
		RXFrequencyMHz: float32(*freq),
		Squelch:        byte(*squelch),
	}

	if err := sendDesired(c, desired); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"Parrot ready on %.3f MHz: RX start >= %d, RX stop <= %d",
		*freq,
		startRSSI,
		stopRSSI,
	)

	var (
		receiving     bool
		packets       [][]byte
		lowRSSICount  int
		cooldownUntil time.Time
	)

	for {
		kissFrame, err := c.ReadFrame()
		if err != nil {
			log.Fatal(err)
		}

		vendor, err := protocol.DecodeVendorFrame(kissFrame)
		if err != nil {
			continue
		}

		switch vendor.Command {
		case protocol.CommandRxAudio:
			if !receiving || time.Now().Before(cooldownUntil) {
				continue
			}

			if len(packets) < maxPackets {
				packets = append(
					packets,
					append([]byte(nil), vendor.Payload...),
				)
			}

		case protocol.CommandDeviceState:
			state, err := protocol.ParseDeviceState(vendor.Payload)
			if err != nil {
				log.Printf("Bad device state: %v", err)
				continue
			}

			if state.LastError != 0 {
				log.Printf("Radio error: %d", state.LastError)
				continue
			}

			if time.Now().Before(cooldownUntil) {
				continue
			}

			rssi := int(state.LatestRSSI)

			if !receiving {
				if rssi >= startRSSI {
					receiving = true
					packets = nil
					lowRSSICount = 0
					log.Printf("RX started: RSSI %d", rssi)
				}
				continue
			}

			if rssi <= stopRSSI {
				lowRSSICount++
			} else {
				lowRSSICount = 0
			}

			if lowRSSICount < stopCount {
				continue
			}

			receiving = false
			lowRSSICount = 0

			if len(packets) < minPackets {
				log.Printf("Ignoring short reception: %d packets", len(packets))
				packets = nil
				continue
			}

			duration := time.Duration(len(packets)) * frameDuration
			log.Printf(
				"RX ended: %d packets, approximately %s",
				len(packets),
				duration,
			)

			captured := packets
			packets = nil

			sequence++

			if err := replay(c, &desired, sequence, captured); err != nil {
				log.Printf("Replay failed: %v", err)

				sequence++
				desired.Sequence = sequence
				desired.Flags &^= protocol.HostStatePTTRequested
				desired.Flags |= protocol.HostStateRXAudioOpen
				_ = sendDesired(c, desired)
			}

			cooldownUntil = time.Now().Add(postTXCooldown)
		}
	}
}

func replay(
	c *client.Client,
	desired *protocol.HostDesiredState,
	sequence uint32,
	packets [][]byte,
) error {
	time.Sleep(preTXDelay)

	desired.Sequence = sequence
	desired.Flags |= protocol.HostStatePTTRequested
	desired.Flags &^= protocol.HostStateRXAudioOpen

	log.Println("PTT down")
	if err := sendDesired(c, *desired); err != nil {
		return err
	}

	time.Sleep(keyupDelay)

	for _, packet := range packets {
		start := time.Now()

		frame := protocol.EncodeVendorFrame(
			protocol.CommandHostTXAudio,
			packet,
		)

		if err := c.Write(frame); err != nil {
			return fmt.Errorf("send audio: %w", err)
		}

		if elapsed := time.Since(start); elapsed < txFramePace {
			time.Sleep(txFramePace - elapsed)
		}
	}

	time.Sleep(postAudioDelay)

	desired.Sequence++
	desired.Flags &^= protocol.HostStatePTTRequested
	desired.Flags |= protocol.HostStateRXAudioOpen

	log.Println("PTT up")
	if err := sendDesired(c, *desired); err != nil {
		return err
	}

	log.Println("Replay completed")
	return nil
}

func sendDesired(c *client.Client, state protocol.HostDesiredState) error {
	frame, err := protocol.EncodeDesiredStateFrame(state)
	if err != nil {
		return err
	}

	return c.Write(frame)
}
