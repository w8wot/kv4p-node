package audio

import (
	"fmt"

	"gopkg.in/hraban/opus.v2"
)

const (
	SampleRate   = 48000
	Channels     = 1
	MaxFrameSize = 5760
)

type Decoder struct {
	opus *opus.Decoder
}

func NewDecoder() (*Decoder, error) {
	decoder, err := opus.NewDecoder(SampleRate, Channels)
	if err != nil {
		return nil, fmt.Errorf("create Opus decoder: %w", err)
	}

	return &Decoder{opus: decoder}, nil
}

func (d *Decoder) Decode(packet []byte) ([]int16, error) {
	pcm := make([]int16, MaxFrameSize)

	n, err := d.opus.Decode(packet, pcm)
	if err != nil {
		return nil, fmt.Errorf("decode Opus packet: %w", err)
	}

	return pcm[:n], nil
}
