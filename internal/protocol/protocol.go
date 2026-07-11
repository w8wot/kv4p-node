package protocol

import (
	"bytes"
	"fmt"

	"github.com/w8wot/kv4p-node/internal/kiss"
)

const (
	ProtocolVersion = 0x01
)

var VendorPrefix = []byte("KV4P")

const (
	CommandHello            = 0x06
	CommandRxAudio          = 0x07
	CommandWindowUpdate     = 0x09
	CommandDeviceState      = 0x0B
	CommandHostDesiredState = 0x0D
)

func EncodeVendorFrame(command byte, payload []byte) []byte {
	body := make([]byte, 0, len(payload)+6)

	body = append(body, VendorPrefix...)
	body = append(body, ProtocolVersion)
	body = append(body, command)
	body = append(body, payload...)

	return kiss.Encode(kiss.CommandSetHardware, body)
}

type VendorFrame struct {
	Command byte
	Payload []byte
}

func DecodeVendorFrame(frame kiss.Frame) (*VendorFrame, error) {

	if frame.Command != kiss.CommandSetHardware {
		return nil, fmt.Errorf("not a vendor frame")
	}

	if len(frame.Payload) < 6 {
		return nil, fmt.Errorf("frame too short")
	}

	if !bytes.Equal(frame.Payload[:4], VendorPrefix) {
		return nil, fmt.Errorf("bad vendor prefix")
	}

	if frame.Payload[4] != ProtocolVersion {
		return nil, fmt.Errorf("unsupported protocol version %d", frame.Payload[4])
	}

	return &VendorFrame{
		Command: frame.Payload[5],
		Payload: frame.Payload[6:],
	}, nil
}
