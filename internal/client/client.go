package client

import (
	"context"
	"io"

	"github.com/w8wot/kv4p-node/internal/kiss"
	"github.com/w8wot/kv4p-node/internal/transport"
)

type Transport interface {
	Read([]byte) (int, error)
	Write([]byte) (int, error)
	Close() error
	ResetDevice() error
}

type Client struct {
	transport Transport
	parser    *kiss.Parser
	pending   []kiss.Frame
}

func New(t Transport) *Client {
	return &Client{
		transport: t,
		parser:    kiss.NewParser(),
	}
}

func Connect(port string) (*Client, error) {
	t, err := transport.Open(port)
	if err != nil {
		return nil, err
	}

	return New(t), nil
}

func (c *Client) Close() error {
	return c.transport.Close()
}

func (c *Client) Write(frame []byte) error {
	written, err := c.transport.Write(frame)
	if err != nil {
		return err
	}
	if written != len(frame) {
		return io.ErrShortWrite
	}

	return nil
}

func (c *Client) ReadFrame() (kiss.Frame, error) {
	return c.ReadFrameContext(context.Background())
}

func (c *Client) ReadFrameContext(ctx context.Context) (kiss.Frame, error) {
	if len(c.pending) > 0 {
		frame := c.pending[0]
		c.pending = c.pending[1:]
		return frame, nil
	}

	buf := make([]byte, 512)

	for {
		if err := ctx.Err(); err != nil {
			return kiss.Frame{}, err
		}

		n, readErr := c.transport.Read(buf)

		if n > 0 {
			frames, err := c.parser.Feed(buf[:n])
			if err != nil {
				return kiss.Frame{}, err
			}

			if len(frames) > 0 {
				if len(frames) > 1 {
					c.pending = append(c.pending, frames[1:]...)
				}
				return frames[0], nil
			}
		}

		if readErr != nil {
			return kiss.Frame{}, readErr
		}

		if err := ctx.Err(); err != nil {
			return kiss.Frame{}, err
		}
	}
}

func (c *Client) ResetDevice() error {
	return c.transport.ResetDevice()
}
