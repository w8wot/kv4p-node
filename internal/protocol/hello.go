package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

const HelloVersionSize = 17
const HelloSize = HelloVersionSize + DeviceStateSize

type Hello struct {
	Version         uint16
	RadioStatus     byte
	WindowSize      uint32
	RFModuleType    byte
	MinFrequencyMHz float32
	MaxFrequencyMHz float32
	Features        byte
	DeviceState     DeviceState
}

func ParseHello(payload []byte) (Hello, error) {
	if len(payload) < HelloSize {
		return Hello{}, fmt.Errorf(
			"hello payload too short: got %d bytes, need %d",
			len(payload),
			HelloSize,
		)
	}

	state, err := ParseDeviceState(payload[HelloVersionSize:])
	if err != nil {
		return Hello{}, fmt.Errorf("parse initial device state: %w", err)
	}

	return Hello{
		Version:      binary.LittleEndian.Uint16(payload[0:2]),
		RadioStatus:  payload[2],
		WindowSize:   binary.LittleEndian.Uint32(payload[3:7]),
		RFModuleType: payload[7],
		MinFrequencyMHz: math.Float32frombits(
			binary.LittleEndian.Uint32(payload[8:12]),
		),
		MaxFrequencyMHz: math.Float32frombits(
			binary.LittleEndian.Uint32(payload[12:16]),
		),
		Features:    payload[16],
		DeviceState: state,
	}, nil
}
