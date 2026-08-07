package radio

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"math"
	"testing"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
)

type fakeTransport struct {
	reads  [][]byte
	writes [][]byte
	index  int
}

func (f *fakeTransport) Read(buf []byte) (int, error) {
	if f.index >= len(f.reads) {
		return 0, io.EOF
	}

	data := f.reads[f.index]
	f.index++

	return copy(buf, data), nil
}

func (f *fakeTransport) Write(data []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), data...))
	return len(data), nil
}

func (f *fakeTransport) Close() error {
	return nil
}

func (f *fakeTransport) ResetDevice() error {
	return nil
}

func encodedDeviceState(t *testing.T, state protocol.DeviceState) []byte {
	t.Helper()

	payload := make([]byte, protocol.DeviceStateSize)

	binary.LittleEndian.PutUint32(payload[0:4], state.AppliedSequence)
	binary.LittleEndian.PutUint32(payload[4:8], uint32(state.MemoryID))
	binary.LittleEndian.PutUint16(payload[8:10], state.Flags)
	payload[10] = state.Bandwidth
	binary.LittleEndian.PutUint32(
		payload[11:15],
		math.Float32bits(state.TXFrequencyMHz),
	)
	binary.LittleEndian.PutUint32(
		payload[15:19],
		math.Float32bits(state.RXFrequencyMHz),
	)
	payload[19] = state.CTCSSTX
	payload[20] = state.Squelch
	payload[21] = state.CTCSSRX
	payload[22] = state.RadioStatus
	payload[23] = state.Mode
	payload[24] = state.LastError
	payload[25] = state.LatestRSSI

	return protocol.EncodeVendorFrame(
		protocol.CommandDeviceState,
		payload,
	)
}

func TestStateControllerApplyWaitsForMatchingSequence(t *testing.T) {
	desired := protocol.HostDesiredState{
		Sequence: 42,
		MemoryID: -1,
		Flags:    protocol.HostStateRXAudioOpen,
	}

	transport := &fakeTransport{
		reads: [][]byte{
			encodedDeviceState(t, protocol.DeviceState{
				AppliedSequence: 41,
			}),
			encodedDeviceState(t, protocol.DeviceState{
				AppliedSequence: 42,
			}),
		},
	}

	controller := NewStateController(client.New(transport), desired)

	if err := controller.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}

	if controller.State.AppliedSequence != 42 {
		t.Fatalf(
			"applied sequence = %d, want 42",
			controller.State.AppliedSequence,
		)
	}

	if len(transport.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(transport.writes))
	}
}

func TestStateControllerSetPTTUpdatesFlagsAndSequence(t *testing.T) {
	desired := protocol.HostDesiredState{
		Sequence: 100,
		MemoryID: -1,
		Flags:    protocol.HostStateRXAudioOpen,
	}

	transport := &fakeTransport{
		reads: [][]byte{
			encodedDeviceState(t, protocol.DeviceState{
				AppliedSequence: 101,
			}),
		},
	}

	controller := NewStateController(client.New(transport), desired)

	if err := controller.SetPTT(context.Background(), true); err != nil {
		t.Fatal(err)
	}

	got := controller.Desired()

	if got.Sequence != 101 {
		t.Fatalf("sequence = %d, want 101", got.Sequence)
	}

	if got.Flags&protocol.HostStatePTTRequested == 0 {
		t.Fatal("PTT requested flag was not set")
	}

	if got.Flags&protocol.HostStateRXAudioOpen != 0 {
		t.Fatal("RX audio open flag was not cleared")
	}

	if len(transport.writes) != 1 {
		t.Fatalf("writes = %d, want 1", len(transport.writes))
	}

	wantFrame, err := protocol.EncodeDesiredStateFrame(got)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(transport.writes[0], wantFrame) {
		t.Fatalf(
			"written frame = % x, want % x",
			transport.writes[0],
			wantFrame,
		)
	}
}

func TestStateControllerApplyRetainsStateWhenRadioReportsError(t *testing.T) {
	desired := protocol.HostDesiredState{
		Sequence: 42,
		MemoryID: -1,
		Flags:    protocol.HostStateRXAudioOpen,
	}

	transport := &fakeTransport{
		reads: [][]byte{
			encodedDeviceState(t, protocol.DeviceState{
				AppliedSequence: 42,
				LastError:       7,
				LatestRSSIValid: true,
				LatestRSSI:      93,
			}),
		},
	}

	controller := NewStateController(client.New(transport), desired)

	err := controller.Apply(context.Background())
	if err == nil {
		t.Fatal("Apply returned nil error, want radio error")
	}

	if controller.State.AppliedSequence != 42 {
		t.Fatalf(
			"applied sequence = %d, want 42",
			controller.State.AppliedSequence,
		)
	}

	if controller.State.LastError != 7 {
		t.Fatalf(
			"last error = %d, want 7",
			controller.State.LastError,
		)
	}

	if controller.State.LatestRSSI != 93 {
		t.Fatalf(
			"RSSI = %d, want 93",
			controller.State.LatestRSSI,
		)
	}
}
