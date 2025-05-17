package main

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidPacket        = errors.New("invalid packet")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrPeerNotFound         = errors.New("peer not found")
	ErrInvalidPublicKey     = errors.New("invalid public key")
	ErrPacketSendFailed     = errors.New("failed to send packet")
)

func NewInvalidPacketError(details string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPacket, details)
}

func NewAuthenticationFailedError(details string) error {
	return fmt.Errorf("%w: %s", ErrAuthenticationFailed, details)
}

func NewPeerNotFoundError(details string) error {
	return fmt.Errorf("%w: %s", ErrPeerNotFound, details)
}

func NewInvalidPublicKeyError(details string) error {
	return fmt.Errorf("%w: %s", ErrInvalidPublicKey, details)
}

func NewPacketSendFailedError(err error) error {
	return fmt.Errorf("%w: %v", ErrPacketSendFailed, err)
}
