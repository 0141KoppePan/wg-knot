package main

import (
	"encoding/hex"
	"net"
)

type PacketSender interface {
	SendPacket(to *net.UDPAddr, payload []byte) error
}

type UDPPacketSender struct {
	conn   *net.UDPConn
	logger LoggerInterface
}

func NewUDPPacketSender(conn *net.UDPConn, logger LoggerInterface) *UDPPacketSender {
	return &UDPPacketSender{conn: conn, logger: logger}
}

func (s *UDPPacketSender) SendPacket(to *net.UDPAddr, payload []byte) error {
	_, err := s.conn.WriteToUDP(payload, to)
	if err == nil {
		s.logger.Debug("Packet sent to %s", to.String())
		s.logger.Debug("Packet: %d byte\n%s", len(payload), hex.Dump(payload))
	}
	return err
}
