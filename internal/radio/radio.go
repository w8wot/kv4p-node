package radio

import (
	"fmt"
	"time"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
	"github.com/w8wot/kv4p-node/internal/transport"
)

const hostFlagMask uint16 = protocol.HostStateRadioConfigValid |
	protocol.HostStatePTTRequested |
	protocol.HostStateRXAudioOpen |
	protocol.HostStateHighPower |
	protocol.HostStateRSSIEnabled |
	protocol.HostStateFilterPre |
	protocol.HostStateFilterHigh |
	protocol.HostStateFilterLow |
	protocol.HostStateTXAllowed |
	protocol.HostStateStatusReports

type Radio struct {
	client   *client.Client
	Hello    protocol.Hello
	State    protocol.DeviceState
	desired  protocol.HostDesiredState
	sequence uint32
}

func Connect(portName string) (*Radio, error) {
	for {
		currentPort := portName

		if currentPort == "" {
			found, err := transport.FindKV4P()
			if err != nil {
				time.Sleep(500 * time.Millisecond)
				continue
			}
			currentPort = found
		}

		c, err := client.Connect(currentPort)
		if err != nil {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		for {
			frame, err := c.ReadFrame()
			if err != nil {
				_ = c.Close()
				time.Sleep(500 * time.Millisecond)
				break
			}

			vendor, err := protocol.DecodeVendorFrame(frame)
			if err != nil || vendor.Command != protocol.CommandHello {
				continue
			}

			hello, err := protocol.ParseHello(vendor.Payload)
			if err != nil {
				_ = c.Close()
				return nil, fmt.Errorf("parse HELLO: %w", err)
			}

			r := &Radio{
				client:   c,
				Hello:    hello,
				State:    hello.DeviceState,
				sequence: hello.DeviceState.AppliedSequence,
				desired: protocol.HostDesiredState{
					Sequence:       hello.DeviceState.AppliedSequence,
					MemoryID:       hello.DeviceState.MemoryID,
					Flags:          hello.DeviceState.Flags & hostFlagMask,
					Bandwidth:      hello.DeviceState.Bandwidth,
					TXFrequencyMHz: hello.DeviceState.TXFrequencyMHz,
					RXFrequencyMHz: hello.DeviceState.RXFrequencyMHz,
					CTCSSTX:        hello.DeviceState.CTCSSTX,
					Squelch:        hello.DeviceState.Squelch,
					CTCSSRX:        hello.DeviceState.CTCSSRX,
				},
			}

			return r, nil
		}
	}
}

func (r *Radio) Close() error {
	return r.client.Close()
}

func (r *Radio) ConfigureReceive(frequencyMHz float32, squelch byte) error {
	if frequencyMHz < r.Hello.MinFrequencyMHz ||
		frequencyMHz > r.Hello.MaxFrequencyMHz {
		return fmt.Errorf(
			"frequency %.3f MHz is outside radio range %.3f-%.3f MHz",
			frequencyMHz,
			r.Hello.MinFrequencyMHz,
			r.Hello.MaxFrequencyMHz,
		)
	}

	if squelch > 8 {
		return fmt.Errorf("squelch must be 0-8")
	}

	r.sequence++

	r.desired.Sequence = r.sequence
	r.desired.MemoryID = -1
	r.desired.TXFrequencyMHz = frequencyMHz
	r.desired.RXFrequencyMHz = frequencyMHz
	r.desired.Squelch = squelch
	r.desired.Flags =
		protocol.HostStateRadioConfigValid |
			protocol.HostStateRXAudioOpen |
			protocol.HostStateRSSIEnabled |
			protocol.HostStateFilterHigh |
			protocol.HostStateFilterLow |
			protocol.HostStateStatusReports

	return r.sendDesiredState()
}

func (r *Radio) sendDesiredState() error {
	frame, err := protocol.EncodeDesiredStateFrame(r.desired)
	if err != nil {
		return err
	}

	if err := r.client.Write(frame); err != nil {
		return fmt.Errorf("write desired state: %w", err)
	}

	for {
		frame, err := r.client.ReadFrame()
		if err != nil {
			return fmt.Errorf("wait for device state: %w", err)
		}

		vendor, err := protocol.DecodeVendorFrame(frame)
		if err != nil || vendor.Command != protocol.CommandDeviceState {
			continue
		}

		state, err := protocol.ParseDeviceState(vendor.Payload)
		if err != nil {
			return err
		}

		r.State = state

		if state.AppliedSequence == r.desired.Sequence {
			return nil
		}
	}
}

func (r *Radio) ReadVendorFrame() (protocol.VendorFrame, error) {
	for {
		frame, err := r.client.ReadFrame()
		if err != nil {
			return protocol.VendorFrame{}, err
		}

		vendor, err := protocol.DecodeVendorFrame(frame)
		if err != nil {
			continue
		}

		return *vendor, nil
	}
}

// ConfigureParrot configures simplex RX/TX operation and permits transmission.
// It does not key the transmitter.
func (r *Radio) ConfigureParrot(frequencyMHz float32, squelch byte) error {
	if frequencyMHz < r.Hello.MinFrequencyMHz ||
		frequencyMHz > r.Hello.MaxFrequencyMHz {
		return fmt.Errorf(
			"frequency %.3f MHz is outside radio range %.3f-%.3f MHz",
			frequencyMHz,
			r.Hello.MinFrequencyMHz,
			r.Hello.MaxFrequencyMHz,
		)
	}

	if squelch > 8 {
		return fmt.Errorf("squelch must be 0-8")
	}

	r.sequence++

	r.desired.Sequence = r.sequence
	r.desired.MemoryID = -1
	r.desired.TXFrequencyMHz = frequencyMHz
	r.desired.RXFrequencyMHz = frequencyMHz
	r.desired.Squelch = squelch
	r.desired.Flags =
		protocol.HostStateRadioConfigValid |
			protocol.HostStateRXAudioOpen |
			protocol.HostStateRSSIEnabled |
			protocol.HostStateFilterHigh |
			protocol.HostStateFilterLow |
			protocol.HostStateTXAllowed |
			protocol.HostStateStatusReports

	return r.sendDesiredState()
}

func (r *Radio) SetPTT(enabled bool) error {
	r.sequence++
	r.desired.Sequence = r.sequence

	if enabled {
		r.desired.Flags |= protocol.HostStatePTTRequested
		r.desired.Flags &^= protocol.HostStateRXAudioOpen
	} else {
		r.desired.Flags &^= protocol.HostStatePTTRequested
		r.desired.Flags |= protocol.HostStateRXAudioOpen
	}

	return r.sendDesiredState()
}

func (r *Radio) SendOpusPacket(packet []byte) error {
	if len(packet) == 0 {
		return fmt.Errorf("cannot send empty Opus packet")
	}

	frame := protocol.EncodeVendorFrame(
		protocol.CommandHostTXAudio,
		packet,
	)

	if err := r.client.Write(frame); err != nil {
		return fmt.Errorf("send Opus packet: %w", err)
	}

	return nil
}
