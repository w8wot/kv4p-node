package transport

import (
	"go.bug.st/serial"
)

type Transport struct {
	port serial.Port
}

func Open(portName string) (*Transport, error) {
	mode := &serial.Mode{
		BaudRate: 115200,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		return nil, err
	}

	return &Transport{
		port: port,
	}, nil
}

func (t *Transport) Close() error {
	return t.port.Close()
}

func (t *Transport) Write(frame []byte) (int, error) {
	return t.port.Write(frame)
}

func (t *Transport) Read(buf []byte) (int, error) {
	return t.port.Read(buf)
}
