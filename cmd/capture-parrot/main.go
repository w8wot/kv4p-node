package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/w8wot/kv4p-node/internal/audio"
	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
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
	output := flag.String("output", "capture-parrot.wav", "Saved receive WAV filename")
	txPace := flag.Duration("tx-pace", txFramePace, "Delay between replayed audio packets")
	flag.Parse()

	decoder, err := audio.NewDecoder()
	if err != nil {
		log.Fatal(err)
	}

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

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)
	defer signal.Stop(interrupt)

	go func() {
		<-interrupt
		log.Println("Interrupt received: releasing PTT and closing radio")

		cleanup := desired
		cleanup.Sequence++
		cleanup.Flags &^= protocol.HostStatePTTRequested
		cleanup.Flags |= protocol.HostStateRXAudioOpen

		_ = sendDesired(c, cleanup)
		_ = c.Close()
		os.Exit(130)
	}()

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

			if state.LastError != protocol.DeviceStateErrorNone {
				log.Printf("Radio error: %s", state.LastError)
				continue
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

			if err := decodeToWAV(decoder, captured, *output); err != nil {
				log.Printf("Save WAV failed: %v", err)
			} else {
				log.Printf("Saved received audio to %s", *output)
			}

			sequence++

			// Replay the exact original Opus packets received from the KV4P.
			if err := replay(c, &desired, sequence, captured, *txPace); err != nil {
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
	txFramePace time.Duration,
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
func decodeToWAV(
	decoder interface {
		Decode([]byte) ([]int16, error)
	},
	packets [][]byte,
	filename string,
) error {
	var pcm []int16

	for index, packet := range packets {
		samples, err := decoder.Decode(packet)
		if err != nil {
			return fmt.Errorf("decode packet %d: %w", index+1, err)
		}

		pcm = append(pcm, samples...)
	}

	if len(pcm) == 0 {
		return fmt.Errorf("no PCM audio was decoded")
	}

	if err := writeWAV(filename, pcm, audio.SampleRate); err != nil {
		return err
	}

	duration := time.Duration(
		float64(len(pcm)) /
			float64(audio.SampleRate) *
			float64(time.Second),
	)

	log.Printf(
		"Decoded %d PCM samples at %d Hz, duration approximately %s",
		len(pcm),
		audio.SampleRate,
		duration,
	)

	return nil
}

func writeWAV(filename string, samples []int16, sampleRate int) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("create WAV: %w", err)
	}
	defer file.Close()

	const (
		channels      = 1
		bitsPerSample = 16
		headerSize    = 44
	)

	dataSize := len(samples) * 2
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	riffSize := 36 + dataSize

	header := make([]byte, headerSize)

	copy(header[0:4], "RIFF")
	binary.LittleEndian.PutUint32(header[4:8], uint32(riffSize))
	copy(header[8:12], "WAVE")
	copy(header[12:16], "fmt ")
	binary.LittleEndian.PutUint32(header[16:20], 16)
	binary.LittleEndian.PutUint16(header[20:22], 1)
	binary.LittleEndian.PutUint16(header[22:24], channels)
	binary.LittleEndian.PutUint32(header[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(header[28:32], uint32(byteRate))
	binary.LittleEndian.PutUint16(header[32:34], uint16(blockAlign))
	binary.LittleEndian.PutUint16(header[34:36], bitsPerSample)
	copy(header[36:40], "data")
	binary.LittleEndian.PutUint32(header[40:44], uint32(dataSize))

	if _, err := file.Write(header); err != nil {
		return fmt.Errorf("write WAV header: %w", err)
	}

	if err := binary.Write(file, binary.LittleEndian, samples); err != nil {
		return fmt.Errorf("write WAV samples: %w", err)
	}

	return nil
}
