package protocol

import (
	"encoding/binary"
	"fmt"
	"math"
)

const HostDesiredStateSize = 22

type HostDesiredState struct {
	Sequence       uint32
	MemoryID       int32
	Flags          uint16
	Bandwidth      byte
	TXFrequencyMHz float32
	RXFrequencyMHz float32
	CTCSSTX        byte
	Squelch        byte
	CTCSSRX        byte
}

func (s HostDesiredState) MarshalBinary() ([]byte, error) {
	if s.Squelch > 8 {
		return nil, fmt.Errorf("invalid squelch level %d: must be 0-8", s.Squelch)
	}

	payload := make([]byte, HostDesiredStateSize)

	binary.LittleEndian.PutUint32(payload[0:4], s.Sequence)
	binary.LittleEndian.PutUint32(payload[4:8], uint32(s.MemoryID))
	binary.LittleEndian.PutUint16(payload[8:10], s.Flags)
	payload[10] = s.Bandwidth
	binary.LittleEndian.PutUint32(payload[11:15], math.Float32bits(s.TXFrequencyMHz))
	binary.LittleEndian.PutUint32(payload[15:19], math.Float32bits(s.RXFrequencyMHz))
	payload[19] = s.CTCSSTX
	payload[20] = s.Squelch
	payload[21] = s.CTCSSRX

	return payload, nil
}

func EncodeDesiredStateFrame(state HostDesiredState) ([]byte, error) {
	payload, err := state.MarshalBinary()
	if err != nil {
		return nil, err
	}

	return EncodeVendorFrame(CommandHostDesiredState, payload), nil
}

func (s HostDesiredState) HasFlag(flag uint16) bool {
	return s.Flags&flag != 0
}
