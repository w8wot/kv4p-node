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

const (
	LegacyDeviceStateSize = 24
	DeviceStateSize       = 26
)

// DeviceStateError values reported by the KV4P firmware.
type DeviceStateError byte

const (
	DeviceStateErrorNone DeviceStateError = iota
	DeviceStateErrorRadioConfigFailed
	DeviceStateErrorFiltersFailed
)

func (e DeviceStateError) String() string {
	switch e {
	case DeviceStateErrorNone:
		return "NONE"
	case DeviceStateErrorRadioConfigFailed:
		return "RADIO_CONFIG_FAILED"
	case DeviceStateErrorFiltersFailed:
		return "FILTERS_FAILED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", byte(e))
	}
}

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
	LastError       DeviceStateError
	LatestRSSI      byte
	LatestRSSIValid bool
}

func ParseDeviceState(payload []byte) (DeviceState, error) {
	if len(payload) < LegacyDeviceStateSize {
		return DeviceState{}, fmt.Errorf(
			"device-state payload too short: got %d bytes, need at least %d",
			len(payload),
			LegacyDeviceStateSize,
		)
	}

	state := DeviceState{
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
	}

	if len(payload) >= DeviceStateSize {
		state.LastError = DeviceStateError(payload[24])
		state.LatestRSSI = payload[25]
		state.LatestRSSIValid = true
	}

	return state, nil
}

func (s DeviceState) HasFlag(flag uint16) bool {
	return s.Flags&flag != 0
}
