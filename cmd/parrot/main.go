package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/radio"
	"github.com/w8wot/kv4p-node/internal/transport"
)

const (
	frameDuration  = 40 * time.Millisecond
	txFramePace    = 40 * time.Millisecond
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

	desired := protocol.HostDesiredState{
		Sequence: uint32(time.Now().Unix()),
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

	controller := radio.NewStateController(c, desired)

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 3*time.Second)
	err = controller.Apply(startupCtx)
	cancelStartup()
	if err != nil {
		log.Fatalf("Confirm startup state: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	defer func() {
		log.Println("Releasing PTT before shutdown")

		cleanupCtx, cancelCleanup := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancelCleanup()

		if err := controller.SetPTT(cleanupCtx, false); err != nil {
			log.Printf("Release PTT during shutdown: %v", err)
		}
	}()

	log.Printf(
		"Parrot ready on %.3f MHz: RX start >= %d, RX stop <= %d",
		*freq,
		startRSSI,
		stopRSSI,
	)

	var (
		receiving      bool
		packets        [][]byte
		lowRSSICount   int
		cooldownUntil  time.Time
		lastRadioError protocol.DeviceStateError
	)

	for {
		select {
		case <-ctx.Done():
			log.Println("Interrupt received: shutting down")
			return
		default:
		}

		kissFrame, err := c.ReadFrameContext(ctx)
		if err != nil {
			if ctx.Err() != nil {
				log.Println("Interrupt received: shutting down")
				return
			}
			log.Printf("Radio read failed: %v", err)
			return
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

			if state.LastError != lastRadioError {
				if state.LastError != protocol.DeviceStateErrorNone {
					log.Printf("Radio error: %s", state.LastError)
				} else if lastRadioError != protocol.DeviceStateErrorNone {
					log.Printf("Radio error cleared: %s", lastRadioError)
				}

				lastRadioError = state.LastError
			}

			if time.Now().Before(cooldownUntil) {
				continue
			}

			if !state.LatestRSSIValid {
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

			if err := replay(c, controller, captured); err != nil {
				log.Printf("Replay failed: %v", err)

				recoveryCtx, cancelRecovery := context.WithTimeout(
					context.Background(),
					3*time.Second,
				)
				_ = controller.SetPTT(recoveryCtx, false)
				cancelRecovery()
			}

			cooldownUntil = time.Now().Add(postTXCooldown)
		}
	}
}

func replay(
	c *client.Client,
	controller *radio.StateController,
	packets [][]byte,
) error {
	time.Sleep(preTXDelay)

	log.Println("PTT down")
	pttDownCtx, cancelPTTDown := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	err := controller.SetPTT(pttDownCtx, true)
	cancelPTTDown()
	if err != nil {
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

	log.Println("PTT up")
	pttUpCtx, cancelPTTUp := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	err = controller.SetPTT(pttUpCtx, false)
	cancelPTTUp()
	if err != nil {
		return err
	}

	log.Println("Replay completed")
	return nil
}
