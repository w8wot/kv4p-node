package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/w8wot/kv4p-node/internal/audio"
	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/transport"
)

const (
	// radioBandwidth controls the FM channel width:
	// 1 = 25 kHz wide FM, standard for normal 2-meter amateur FM
	// 0 = 12.5 kHz narrow FM; change only when narrow operation is required
	radioBandwidth byte = 1

	startRSSI = 80
	stopRSSI  = 60
	stopCount = 3

	maxPackets = 750
	minPackets = 3
)

func main() {
	freq := flag.Float64("freq", 146.520, "Receive frequency in MHz")
	squelch := flag.Int("squelch", 3, "Squelch level 0-8")
	output := flag.String("output", "kv4p-capture.wav", "Output WAV filename")
	flag.Parse()

	if *squelch < 0 || *squelch > 8 {
		log.Fatal("squelch must be between 0 and 8")
	}

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
			protocol.HostStateRSSIEnabled |
			protocol.HostStateFilterHigh |
			protocol.HostStateFilterLow |
			protocol.HostStateStatusReports,
		Bandwidth:      radioBandwidth,
		TXFrequencyMHz: float32(*freq),
		RXFrequencyMHz: float32(*freq),
		Squelch:        byte(*squelch),
	}

	if err := sendDesiredAndWait(c, desired); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"Radio configuration applied: bandwidth=%d frequency=%.3f MHz",
		desired.Bandwidth,
		*freq,
	)

	desired.Sequence++
	desired.Flags |= protocol.HostStateRXAudioOpen

	if err := sendDesiredAndWait(c, desired); err != nil {
		log.Fatal(err)
	}

	log.Printf(
		"Waiting on %.3f MHz without reset: RX start >= %d, RX stop <= %d",
		*freq,
		startRSSI,
		stopRSSI,
	)

	var (
		receiving    bool
		lowRSSICount int
		packets      [][]byte
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
			if !receiving {
				continue
			}

			packetNumber := len(packets) + 1

			if packetNumber <= 10 || len(vendor.Payload) <= 10 {
				previewLength := len(vendor.Payload)
				if previewLength > 24 {
					previewLength = 24
				}

				log.Printf(
					"Opus packet %d: length=%d bytes, first=% x",
					packetNumber,
					len(vendor.Payload),
					vendor.Payload[:previewLength],
				)
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
			rssi := int(state.LatestRSSI)

			if !receiving {
				if rssi >= startRSSI {
					receiving = true
					lowRSSICount = 0
					packets = nil
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
				log.Printf(
					"Ignoring short reception: %d packets",
					len(packets),
				)
				packets = nil
				continue
			}

			log.Printf("RX ended: %d packets", len(packets))

			if err := decodeToWAV(decoder, packets, *output); err != nil {
				log.Fatal(err)
			}

			log.Printf("Saved received audio to %s", *output)
			return
		}
	}
}

func sendDesiredAndWait(
	c *client.Client,
	state protocol.HostDesiredState,
) error {
	frame, err := protocol.EncodeDesiredStateFrame(state)
	if err != nil {
		return fmt.Errorf("encode desired state: %w", err)
	}

	const maxAttempts = 3

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := c.Write(frame); err != nil {
			return fmt.Errorf("send desired state: %w", err)
		}

		stateReports := 0
		lastError := protocol.DeviceStateErrorNone

		for stateReports < 25 {
			kissFrame, err := c.ReadFrame()
			if err != nil {
				return fmt.Errorf("wait for desired state: %w", err)
			}

			vendor, err := protocol.DecodeVendorFrame(kissFrame)
			if err != nil ||
				vendor.Command != protocol.CommandDeviceState {
				continue
			}

			applied, err := protocol.ParseDeviceState(vendor.Payload)
			if err != nil {
				continue
			}

			stateReports++
			lastError = applied.LastError

			if applied.LastError != protocol.DeviceStateErrorNone {
				continue
			}

			if applied.AppliedSequence != state.Sequence {
				continue
			}

			if applied.Bandwidth != state.Bandwidth ||
				applied.TXFrequencyMHz != state.TXFrequencyMHz ||
				applied.RXFrequencyMHz != state.RXFrequencyMHz ||
				applied.Squelch != state.Squelch {
				continue
			}

			return nil
		}

		log.Printf(
			"Desired state not applied; retry %d/%d, last radio error=%s",
			attempt,
			maxAttempts,
			lastError,
		)
	}

	return fmt.Errorf(
		"radio did not apply desired state after %d attempts",
		maxAttempts,
	)
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
