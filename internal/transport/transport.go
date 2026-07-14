package transport

import (
	"time"

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

	if err := port.SetReadTimeout(250 * time.Millisecond); err != nil {
		_ = port.Close()
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

func (t *Transport) ResetDevice() error {
	if err := t.port.SetDTR(false); err != nil {
		return err
	}
	if err := t.port.SetRTS(true); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	if err := t.port.SetDTR(true); err != nil {
		return err
	}
	if err := t.port.SetRTS(false); err != nil {
		return err
	}

	time.Sleep(100 * time.Millisecond)

	if err := t.port.SetDTR(false); err != nil {
		return err
	}
	return t.port.SetRTS(true)
}
