package radio

import (
	"context"
	"fmt"

	"github.com/w8wot/kv4p-node/internal/client"
	"github.com/w8wot/kv4p-node/internal/protocol"
)

type StateController struct {
	client  *client.Client
	desired protocol.HostDesiredState
	State   protocol.DeviceState
}

func NewStateController(
	c *client.Client,
	desired protocol.HostDesiredState,
) *StateController {
	return &StateController{
		client:  c,
		desired: desired,
	}
}

func (s *StateController) Desired() protocol.HostDesiredState {
	return s.desired
}

func (s *StateController) Apply(ctx context.Context) error {
	frame, err := protocol.EncodeDesiredStateFrame(s.desired)
	if err != nil {
		return err
	}

	if err := s.client.Write(frame); err != nil {
		return fmt.Errorf("write desired state: %w", err)
	}

	for {
		frame, err := s.client.ReadFrameContext(ctx)
		if err != nil {
			return fmt.Errorf("wait for applied state: %w", err)
		}

		vendor, err := protocol.DecodeVendorFrame(frame)
		if err != nil || vendor.Command != protocol.CommandDeviceState {
			continue
		}

		state, err := protocol.ParseDeviceState(vendor.Payload)
		if err != nil {
			return fmt.Errorf("parse device state: %w", err)
		}

		s.State = state

		if state.LastError != protocol.DeviceStateErrorNone {
			return fmt.Errorf("radio reported error %s", state.LastError)
		}

		if state.AppliedSequence == s.desired.Sequence {
			return nil
		}
	}
}

func (s *StateController) Update(
	ctx context.Context,
	update func(*protocol.HostDesiredState),
) error {
	s.desired.Sequence++
	update(&s.desired)
	return s.Apply(ctx)
}

func (s *StateController) SetPTT(ctx context.Context, enabled bool) error {
	return s.Update(ctx, func(desired *protocol.HostDesiredState) {
		if enabled {
			desired.Flags |= protocol.HostStatePTTRequested
			desired.Flags &^= protocol.HostStateRXAudioOpen
			return
		}

		desired.Flags &^= protocol.HostStatePTTRequested
		desired.Flags |= protocol.HostStateRXAudioOpen
	})
}
