package protocol

import (
	"bytes"
	"testing"

	"github.com/w8wot/kv4p-node/internal/kiss"
)

func TestVendorRoundTrip(t *testing.T) {

	encoded := EncodeVendorFrame(
		CommandHello,
		[]byte{1, 2, 3, 4},
	)

	parser := kiss.NewParser()

	frames, err := parser.Feed(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if len(frames) != 1 {
		t.Fatal("expected one frame")
	}

	v, err := DecodeVendorFrame(frames[0])
	if err != nil {
		t.Fatal(err)
	}

	if v.Command != CommandHello {
		t.Fatal("wrong command")
	}

	if !bytes.Equal(v.Payload, []byte{1, 2, 3, 4}) {
		t.Fatal("payload mismatch")
	}
}
