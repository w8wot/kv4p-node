package client

import (
	"bytes"
	"io"
	"testing"

	"github.com/w8wot/kv4p-node/internal/kiss"
)

type fakeTransport struct {
	reads      [][]byte
	writes     [][]byte
	readIndex  int
	closed     bool
	resetCalls int
}

func (f *fakeTransport) Read(buf []byte) (int, error) {
	if f.readIndex >= len(f.reads) {
		return 0, io.EOF
	}

	data := f.reads[f.readIndex]
	f.readIndex++

	n := copy(buf, data)
	return n, nil
}

func (f *fakeTransport) Write(data []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), data...))
	return len(data), nil
}

func (f *fakeTransport) Close() error {
	f.closed = true
	return nil
}

func (f *fakeTransport) ResetDevice() error {
	f.resetCalls++
	return nil
}

func TestReadFrameHandlesSplitReads(t *testing.T) {
	payload := []byte{'K', 'V', '4', 'P', kiss.FEND, kiss.FESC}
	encoded := kiss.Encode(kiss.CommandSetHardware, payload)

	transport := &fakeTransport{
		reads: [][]byte{
			encoded[:4],
			encoded[4:],
		},
	}

	client := New(transport)

	frame, err := client.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}

	if frame.Command != kiss.CommandSetHardware {
		t.Fatalf("command = 0x%02x, want 0x%02x", frame.Command, kiss.CommandSetHardware)
	}

	if !bytes.Equal(frame.Payload, payload) {
		t.Fatalf("payload = % x, want % x", frame.Payload, payload)
	}
}

func TestReadFrameQueuesAdditionalFrames(t *testing.T) {
	firstPayload := []byte{0x01, 0x02}
	secondPayload := []byte{0x03, 0x04}

	combined := append(
		kiss.Encode(kiss.CommandData, firstPayload),
		kiss.Encode(kiss.CommandSetHardware, secondPayload)...,
	)

	transport := &fakeTransport{
		reads: [][]byte{combined},
	}

	client := New(transport)

	first, err := client.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}

	second, err := client.ReadFrame()
	if err != nil {
		t.Fatal(err)
	}

	if first.Command != kiss.CommandData {
		t.Fatalf("first command = 0x%02x, want 0x%02x", first.Command, kiss.CommandData)
	}

	if !bytes.Equal(first.Payload, firstPayload) {
		t.Fatalf("first payload = % x, want % x", first.Payload, firstPayload)
	}

	if second.Command != kiss.CommandSetHardware {
		t.Fatalf("second command = 0x%02x, want 0x%02x", second.Command, kiss.CommandSetHardware)
	}

	if !bytes.Equal(second.Payload, secondPayload) {
		t.Fatalf("second payload = % x, want % x", second.Payload, secondPayload)
	}

	if transport.readIndex != 1 {
		t.Fatalf("transport reads = %d, want 1", transport.readIndex)
	}
}

type shortWriteTransport struct {
	fakeTransport
}

func (s *shortWriteTransport) Write(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	return len(data) - 1, nil
}

func TestWriteReturnsShortWrite(t *testing.T) {
	transport := &shortWriteTransport{}
	client := New(transport)

	err := client.Write([]byte{0x01, 0x02, 0x03})
	if err != io.ErrShortWrite {
		t.Fatalf("Write() error = %v, want %v", err, io.ErrShortWrite)
	}
}

type readErrorTransport struct {
	fakeTransport
	err error
}

func (r *readErrorTransport) Read([]byte) (int, error) {
	return 0, r.err
}

func TestReadFrameReturnsTransportError(t *testing.T) {
	wantErr := io.ErrUnexpectedEOF
	transport := &readErrorTransport{err: wantErr}
	client := New(transport)

	_, err := client.ReadFrame()
	if err != wantErr {
		t.Fatalf("ReadFrame() error = %v, want %v", err, wantErr)
	}
}
