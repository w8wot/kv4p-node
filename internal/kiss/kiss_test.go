package kiss

import (
	"bytes"
	"testing"
)

func TestEncodeEscapesSpecialBytes(t *testing.T) {
	payload := []byte{0x01, FEND, 0x02, FESC, 0x03}

	got := Encode(CommandSetHardware, payload)
	want := []byte{
		FEND,
		CommandSetHardware,
		0x01,
		FESC, TFEND,
		0x02,
		FESC, TFESC,
		0x03,
		FEND,
	}

	if !bytes.Equal(got, want) {
		t.Fatalf("Encode() = % x, want % x", got, want)
	}
}

func TestParserHandlesSplitSerialReads(t *testing.T) {
	payload := []byte{'K', 'V', '4', 'P', 0x01, 0x0D, FEND, FESC}
	encoded := Encode(CommandSetHardware, payload)

	parser := NewParser()

	first, err := parser.Feed(encoded[:4])
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 0 {
		t.Fatalf("received a frame before it was complete")
	}

	second, err := parser.Feed(encoded[4:])
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 {
		t.Fatalf("received %d frames, want 1", len(second))
	}

	if second[0].Command != CommandSetHardware {
		t.Fatalf("command = 0x%02x", second[0].Command)
	}

	if !bytes.Equal(second[0].Payload, payload) {
		t.Fatalf("payload = % x, want % x", second[0].Payload, payload)
	}
}

func TestParserHandlesMultipleFrames(t *testing.T) {
	combined := append(
		Encode(CommandData, []byte{0x01, 0x02}),
		Encode(CommandSetHardware, []byte{0x03, 0x04})...,
	)

	parser := NewParser()
	frames, err := parser.Feed(combined)
	if err != nil {
		t.Fatal(err)
	}

	if len(frames) != 2 {
		t.Fatalf("received %d frames, want 2", len(frames))
	}
}
