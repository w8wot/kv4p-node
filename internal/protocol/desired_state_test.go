package protocol

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/w8wot/kv4p-node/internal/kiss"
)

func TestMarshalHostDesiredState(t *testing.T) {
	state := HostDesiredState{
		Sequence: 42,
		MemoryID: -1,
		Flags: HostStateRadioConfigValid |
			HostStateRXAudioOpen |
			HostStateTXAllowed |
			HostStateStatusReports,
		Bandwidth:      1,
		TXFrequencyMHz: 146.520,
		RXFrequencyMHz: 146.520,
		Squelch:        3,
	}

	payload, err := state.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if len(payload) != HostDesiredStateSize {
		t.Fatalf("payload length = %d, want %d", len(payload), HostDesiredStateSize)
	}

	if binary.LittleEndian.Uint32(payload[0:4]) != 42 {
		t.Fatal("sequence was encoded incorrectly")
	}

	if int32(binary.LittleEndian.Uint32(payload[4:8])) != -1 {
		t.Fatal("memory ID was encoded incorrectly")
	}

	if binary.LittleEndian.Uint16(payload[8:10]) != state.Flags {
		t.Fatal("flags were encoded incorrectly")
	}

	tx := math.Float32frombits(binary.LittleEndian.Uint32(payload[11:15]))
	rx := math.Float32frombits(binary.LittleEndian.Uint32(payload[15:19]))

	if tx != state.TXFrequencyMHz || rx != state.RXFrequencyMHz {
		t.Fatalf("frequencies = %.3f/%.3f", tx, rx)
	}
}

func TestEncodeDesiredStateFrame(t *testing.T) {
	state := HostDesiredState{
		Sequence:       1,
		MemoryID:       -1,
		Flags:          HostStateRadioConfigValid | HostStateRXAudioOpen,
		Bandwidth:      1,
		TXFrequencyMHz: 146.520,
		RXFrequencyMHz: 146.520,
		Squelch:        2,
	}

	encoded, err := EncodeDesiredStateFrame(state)
	if err != nil {
		t.Fatal(err)
	}

	parser := kiss.NewParser()
	frames, err := parser.Feed(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if len(frames) != 1 {
		t.Fatalf("received %d frames, want 1", len(frames))
	}

	vendor, err := DecodeVendorFrame(frames[0])
	if err != nil {
		t.Fatal(err)
	}

	if vendor.Command != CommandHostDesiredState {
		t.Fatalf("command = 0x%02x, want 0x%02x",
			vendor.Command,
			CommandHostDesiredState,
		)
	}

	if len(vendor.Payload) != HostDesiredStateSize {
		t.Fatalf("payload length = %d", len(vendor.Payload))
	}
}

func TestDesiredStateRejectsInvalidSquelch(t *testing.T) {
	state := HostDesiredState{Squelch: 9}

	_, err := state.MarshalBinary()
	if err == nil {
		t.Fatal("expected invalid-squelch error")
	}
}
