package protocol

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestParseHello(t *testing.T) {
	payload := make([]byte, HelloSize)

	binary.LittleEndian.PutUint16(payload[0:2], 22)
	payload[2] = 'R'
	binary.LittleEndian.PutUint32(payload[3:7], 4096)
	payload[7] = 1
	binary.LittleEndian.PutUint32(payload[8:12], math.Float32bits(134.0))
	binary.LittleEndian.PutUint32(payload[12:16], math.Float32bits(174.0))
	payload[16] = 0x03

	state := payload[HelloVersionSize:]
	binary.LittleEndian.PutUint32(state[0:4], 7)
	binary.LittleEndian.PutUint32(state[4:8], uint32(0xFFFFFFFF))
	binary.LittleEndian.PutUint16(
		state[8:10],
		HostStateRadioConfigValid|HostStateRXAudioOpen,
	)
	state[10] = 1
	binary.LittleEndian.PutUint32(state[11:15], math.Float32bits(146.520))
	binary.LittleEndian.PutUint32(state[15:19], math.Float32bits(146.520))
	state[20] = 3
	state[22] = 'R'
	state[23] = 4
	state[25] = 128

	hello, err := ParseHello(payload)
	if err != nil {
		t.Fatal(err)
	}

	if hello.Version != 22 {
		t.Fatalf("version = %d, want 22", hello.Version)
	}

	if hello.WindowSize != 4096 {
		t.Fatalf("window = %d, want 4096", hello.WindowSize)
	}

	if hello.MinFrequencyMHz != 134.0 || hello.MaxFrequencyMHz != 174.0 {
		t.Fatalf(
			"frequency range = %.3f-%.3f",
			hello.MinFrequencyMHz,
			hello.MaxFrequencyMHz,
		)
	}

	if hello.DeviceState.MemoryID != -1 {
		t.Fatalf("memory ID = %d, want -1", hello.DeviceState.MemoryID)
	}

	if !hello.DeviceState.HasFlag(HostStateRXAudioOpen) {
		t.Fatal("RX audio flag was not parsed")
	}

	if hello.DeviceState.LatestRSSI != 128 {
		t.Fatalf("RSSI = %d, want 128", hello.DeviceState.LatestRSSI)
	}
}

func TestParseHelloRejectsShortPayload(t *testing.T) {
	_, err := ParseHello(make([]byte, HelloSize-1))
	if err == nil {
		t.Fatal("expected short-payload error")
	}
}
