package kiss

import "fmt"

const (
	FEND  byte = 0xC0
	FESC  byte = 0xDB
	TFEND byte = 0xDC
	TFESC byte = 0xDD

	CommandData        byte = 0x00
	CommandSetHardware byte = 0x06
)

// Encode builds one complete KISS frame.
func Encode(command byte, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+3)
	frame = append(frame, FEND)
	frame = appendEscaped(frame, command)

	for _, b := range payload {
		frame = appendEscaped(frame, b)
	}

	frame = append(frame, FEND)
	return frame
}

func appendEscaped(dst []byte, b byte) []byte {
	switch b {
	case FEND:
		return append(dst, FESC, TFEND)
	case FESC:
		return append(dst, FESC, TFESC)
	default:
		return append(dst, b)
	}
}

// Parser accepts arbitrary serial chunks and returns complete decoded frames.
type Parser struct {
	inFrame bool
	escaped bool
	drop    bool
	buffer  []byte
}

type Frame struct {
	Command byte
	Payload []byte
}

func NewParser() *Parser {
	return &Parser{
		buffer: make([]byte, 0, 2048),
	}
}

func (p *Parser) Feed(data []byte) ([]Frame, error) {
	var frames []Frame

	for _, b := range data {
		if b == FEND {
			if p.inFrame && len(p.buffer) > 0 && !p.drop {
				frame, err := p.finishFrame()
				if err != nil {
					p.reset(true)
					return frames, err
				}
				frames = append(frames, frame)
			}

			p.reset(true)
			continue
		}

		if !p.inFrame || p.drop {
			continue
		}

		if p.escaped {
			switch b {
			case TFEND:
				p.buffer = append(p.buffer, FEND)
			case TFESC:
				p.buffer = append(p.buffer, FESC)
			default:
				p.drop = true
			}

			p.escaped = false
			continue
		}

		if b == FESC {
			p.escaped = true
			continue
		}

		p.buffer = append(p.buffer, b)
	}

	return frames, nil
}

func (p *Parser) finishFrame() (Frame, error) {
	if len(p.buffer) < 1 {
		return Frame{}, fmt.Errorf("KISS frame has no command byte")
	}

	payload := make([]byte, len(p.buffer)-1)
	copy(payload, p.buffer[1:])

	return Frame{
		Command: p.buffer[0],
		Payload: payload,
	}, nil
}

func (p *Parser) reset(startFrame bool) {
	p.inFrame = startFrame
	p.escaped = false
	p.drop = false
	p.buffer = p.buffer[:0]
}
