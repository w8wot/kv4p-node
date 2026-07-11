package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

const (
	HostStateRadioConfigValid  uint16 = 1 << 0
	HostStatePTTRequested      uint16 = 1 << 1
	HostStateRXAudioOpen       uint16 = 1 << 2
	HostStateHighPower         uint16 = 1 << 3
	HostStateRSSIEnabled       uint16 = 1 << 4
	HostStateFilterPre         uint16 = 1 << 5
	HostStateFilterHigh        uint16 = 1 << 6
	HostStateFilterLow         uint16 = 1 << 7
	DeviceStatePhysicalPTTDown uint16 = 1 << 8
	DeviceStateTXActive        uint16 = 1 << 9
	DeviceStateSquelched       uint16 = 1 << 10
	HostStateTXAllowed         uint16 = 1 << 11
	HostStateStatusReports     uint16 = 1 << 12
)

const DeviceStateSize = 26

type DeviceState struct {
	AppliedSequence uint32
	MemoryID        int32
	Flags           uint16
	Bandwidth       byte
	TXFrequencyMHz  float32
	RXFrequencyMHz  float32
	CTCSSTX         byte
	Squelch         byte
	CTCSSRX         byte
	RadioStatus     byte
	Mode            byte
	LastError       byte
	LatestRSSI      byte
}

func ParseDeviceState(payload []byte) (DeviceState, error) {
	if len(payload) < DeviceStateSize {
		return DeviceState{}, fmt.Errorf(
			"device-state payload too short: got %d bytes, need %d",
			len(payload),
			DeviceStateSize,
		)
	}

	return DeviceState{
		AppliedSequence: binary.LittleEndian.Uint32(payload[0:4]),
		MemoryID:        int32(binary.LittleEndian.Uint32(payload[4:8])),
		Flags:           binary.LittleEndian.Uint16(payload[8:10]),
		Bandwidth:       payload[10],
		TXFrequencyMHz: math.Float32frombits(
			binary.LittleEndian.Uint32(payload[11:15]),
		),
		RXFrequencyMHz: math.Float32frombits(
			binary.LittleEndian.Uint32(payload[15:19]),
		),
		CTCSSTX:     payload[19],
		Squelch:     payload[20],
		CTCSSRX:     payload[21],
		RadioStatus: payload[22],
		Mode:        payload[23],
		LastError:   payload[24],
		LatestRSSI:  payload[25],
	}, nil
}

func (s DeviceState) HasFlag(flag uint16) bool {
	return s.Flags&flag != 0
}
