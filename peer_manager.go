package main

import (
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/blake2s"
)

const WGLabelMAC1 = "mac1----"

const (
	MessageTypeInitiation  = 1
	MessageTypeResponse    = 2
	MessageTypeCookieReply = 3
	MessageTypeTransport   = 4
)

type PublicKey [32]byte
type Mac1Key [32]byte
type SenderID [4]byte
type ReceiverID [4]byte

type Peer struct {
	Addr      *net.UDPAddr
	Timestamp time.Time
}

type PublicKeyPair struct {
	PublicKey1 PublicKey
	PublicKey2 PublicKey
}

type PeerManager struct {
	sync.Mutex
	packetSender                 PacketSender
	ReceiverToPeerMap            map[ReceiverID]*Peer
	PublicKeyToPeersMap          map[PublicKey][]*Peer
	PublicKeyToMac1KeyMap        map[PublicKey]Mac1Key
	PublicKeyToPairPublicKeysMap map[PublicKey][]PublicKey
	logger                       LoggerInterface
	peerExpiration               time.Duration
}

func NewPeerManager(packetSender PacketSender, publicKeyPairList []PublicKeyPair, logger LoggerInterface, peerExpiration time.Duration) *PeerManager {
	pm := &PeerManager{
		packetSender:                 packetSender,
		PublicKeyToPairPublicKeysMap: make(map[PublicKey][]PublicKey),
		PublicKeyToMac1KeyMap:        make(map[PublicKey]Mac1Key),
		PublicKeyToPeersMap:          make(map[PublicKey][]*Peer),
		ReceiverToPeerMap:            make(map[ReceiverID]*Peer),
		logger:                       logger,
		peerExpiration:               peerExpiration,
	}

	for _, publicKeyPair := range publicKeyPairList {
		if _, err := pm.AddPublicKeyPair(context.Background(), publicKeyPair.PublicKey1, publicKeyPair.PublicKey2); err != nil {
			pm.logger.Error("Failed to add public key pair: %v", err)
		}
	}

	return pm
}

func (pm *PeerManager) AddPublicKeyPair(ctx context.Context, publicKey1, publicKey2 PublicKey) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	pm.Lock()
	defer pm.Unlock()

	mac1Key1, err := CalculateMac1Key(publicKey1)
	if err != nil {
		return false, err
	}

	mac1Key2, err := CalculateMac1Key(publicKey2)
	if err != nil {
		return false, err
	}

	pm.PublicKeyToMac1KeyMap[publicKey1] = mac1Key1
	pm.PublicKeyToMac1KeyMap[publicKey2] = mac1Key2

	isEqual := func(a, b PublicKey) bool {
		return a == b
	}

	AppendUniqueValue(pm.PublicKeyToPairPublicKeysMap, publicKey1, publicKey2, isEqual)
	AppendUniqueValue(pm.PublicKeyToPairPublicKeysMap, publicKey2, publicKey1, isEqual)

	return true, nil
}

func (pm *PeerManager) HandlePacket(ctx context.Context, addr *net.UDPAddr, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if len(payload) < 1 {
		return NewInvalidPacketError("insufficient length")
	}

	typeByte := payload[0]
	switch typeByte {
	case MessageTypeInitiation:
		pm.logger.Debug("Received Type1 packet: size=%d bytes", len(payload))

		if len(payload) != 148 {
			return NewInvalidPacketError("invalid Type1 packet length")
		}

		publicKey, err := pm.CheckMAC1AndGetPublicKey(ctx, payload)
		if err != nil {
			return err
		}

		return pm.HandleType1Packet(ctx, addr, SenderID(payload[4:8]), *publicKey, payload)

	case MessageTypeResponse:
		pm.logger.Debug("Received Type2 packet: size=%d bytes", len(payload))

		if len(payload) != 92 {
			return NewInvalidPacketError("invalid Type2 packet length")
		}

		publicKey, err := pm.CheckMAC1AndGetPublicKey(ctx, payload)
		if err != nil {
			return err
		}

		return pm.HandleType2Packet(ctx, addr, SenderID(payload[4:8]), ReceiverID(payload[8:12]), *publicKey, payload)

	case MessageTypeCookieReply:
		pm.logger.Debug("Received Type3 packet: size=%d bytes", len(payload))

		if len(payload) != 64 {
			return NewInvalidPacketError("invalid Type3 packet length")
		}

		return pm.HandleType3And4Packet(ctx, ReceiverID(payload[4:8]), payload)

	case MessageTypeTransport:
		pm.logger.Debug("Received Type4 packet: size=%d bytes", len(payload))

		if len(payload) < 32 {
			return NewInvalidPacketError("invalid Type4 packet length")
		}

		return pm.HandleType3And4Packet(ctx, ReceiverID(payload[4:8]), payload)

	default:
		return NewInvalidPacketError("unknown packet type")
	}
}

// HandleType1Packet handle a Handshake Initiation packet
func (pm *PeerManager) HandleType1Packet(ctx context.Context, addr *net.UDPAddr, senderID SenderID, publicKey PublicKey, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := pm.AddPeerByPublicKey(ctx, addr, senderID, publicKey); err != nil {
		return err
	}

	peers, exists, err := pm.GetPublicKeyToPeers(ctx, publicKey)
	if err != nil {
		return err
	}

	if exists {
		for _, peer := range peers {
			if err := pm.ForwardPacket(ctx, peer.Addr, payload); err != nil {
				return err
			}
		}
	}

	return nil
}

// HandleType2Packet handle a Handshake Response packet
func (pm *PeerManager) HandleType2Packet(ctx context.Context, addr *net.UDPAddr, senderID SenderID, receiverID ReceiverID, publicKey PublicKey, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	pm.logger.Debug("Packet\n%s\n", hex.Dump(payload))

	if err := pm.AddPeerBySenderID(ctx, addr, senderID, publicKey); err != nil {
		return err
	}

	return pm.ForwardPacketToReceiver(ctx, receiverID, payload)
}

// HandleType3And4Packet handle a Cookie Reply and Transport Data packet
func (pm *PeerManager) HandleType3And4Packet(ctx context.Context, receiverID ReceiverID, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	return pm.ForwardPacketToReceiver(ctx, receiverID, payload)
}

func (pm *PeerManager) CheckMAC1AndGetPublicKey(ctx context.Context, payload []byte) (*PublicKey, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	pm.Lock()
	defer pm.Unlock()

	size := len(payload)
	startMac2Pos := size - blake2s.Size128
	startMac1Pos := startMac2Pos - blake2s.Size128
	var mac1 [blake2s.Size128]byte

	for publicKey, mac1Key := range pm.PublicKeyToMac1KeyMap {
		mac, err := blake2s.New128(mac1Key[:])
		if err != nil {
			return nil, err
		}

		mac.Write(payload[:startMac1Pos])
		mac.Sum(mac1[:0])
		if hmac.Equal(mac1[:], payload[startMac1Pos:startMac2Pos]) {
			return &publicKey, nil
		}
	}

	return nil, NewAuthenticationFailedError("mac1 verification failed")
}

func (pm *PeerManager) AddPeerByPublicKey(ctx context.Context, addr *net.UDPAddr, senderID SenderID, receiverPublicKey PublicKey) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	pm.Lock()
	defer pm.Unlock()

	peer, exists := pm.ReceiverToPeerMap[ReceiverID(senderID)]
	if !exists {
		publicKey, exists := pm.PublicKeyToPairPublicKeysMap[receiverPublicKey]
		if !exists {
			return NewPeerNotFoundError("paired public key not found")
		}

		peer = &Peer{Addr: addr, Timestamp: time.Now()}
		isEqual := func(a, b *Peer) bool {
			if a == nil || b == nil {
				return false
			}
			return a.Addr.String() == b.Addr.String()
		}

		if len(publicKey) == 1 {
			AppendUniqueValue(pm.PublicKeyToPeersMap, publicKey[0], peer, isEqual)
			pm.logger.Debug("SenderID: %x, Add peer: %s, PublicKey: %s", senderID, peer.Addr.String(), base64.StdEncoding.EncodeToString(publicKey[0][:]))
		} else {
			pm.logger.Debug(fmt.Sprintf("multiple paired public keys found: %s", base64.StdEncoding.EncodeToString(receiverPublicKey[:])))
		}
	}

	pm.logger.Debug("SenderID: %x, Update peer: %s", senderID, peer.Addr.String())
	pm.ReceiverToPeerMap[ReceiverID(senderID)] = peer

	return nil
}

func (pm *PeerManager) AddPeerBySenderID(ctx context.Context, addr *net.UDPAddr, senderID SenderID, publicKey PublicKey) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	pm.Lock()
	defer pm.Unlock()

	peer, exists := pm.ReceiverToPeerMap[ReceiverID(senderID)]
	if !exists {
		peer = &Peer{Addr: addr, Timestamp: time.Now()}
		pm.logger.Debug("SenderID: %x, Add peer: %s, PublicKey: %s", senderID, peer.Addr.String(), base64.StdEncoding.EncodeToString(publicKey[:]))
		pm.ReceiverToPeerMap[ReceiverID(senderID)] = peer
	}

	return nil
}

func (pm *PeerManager) GetPublicKeyToPeers(ctx context.Context, publicKey PublicKey) ([]*Peer, bool, error) {
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	pm.Lock()
	defer pm.Unlock()

	peers, exists := pm.PublicKeyToPeersMap[publicKey]
	return peers, exists, nil
}

func (pm *PeerManager) ForwardPacketToReceiver(ctx context.Context, receiverID ReceiverID, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	pm.Lock()
	defer pm.Unlock()

	peer, exists := pm.ReceiverToPeerMap[receiverID]
	if !exists {
		return NewPeerNotFoundError(fmt.Sprintf("no peer found for receiver ID: %x", receiverID))
	}

	return pm.ForwardPacket(ctx, peer.Addr, payload)
}

func (pm *PeerManager) ForwardPacket(ctx context.Context, to *net.UDPAddr, payload []byte) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err := pm.packetSender.SendPacket(to, payload); err != nil {
		return NewPacketSendFailedError(err)
	}

	pm.logger.Debug("packet forwarded: destination=%s, size=%d bytes", to.String(), len(payload))
	return nil
}

func (pm *PeerManager) CleanupPeers() error {
	pm.Lock()
	defer pm.Unlock()

	now := time.Now()
	expire := pm.peerExpiration

	if expire <= 0 {
		return fmt.Errorf("invalid peer expiration duration: %v", expire)
	}

	for publicKey, peers := range pm.PublicKeyToPeersMap {
		remaining := make([]*Peer, 0, len(peers))
		for _, peer := range peers {
			if now.Sub(peer.Timestamp) < expire {
				remaining = append(remaining, peer)
			} else {
				pm.logger.Debug("Remove peer from PublicKeyToPeersMap: %s", peer.Addr.String())
			}
		}
		if len(remaining) == 0 {
			pm.logger.Debug("Remove key from PublicKeyToPeersMap: %s", base64.StdEncoding.EncodeToString(publicKey[:]))
			delete(pm.PublicKeyToPeersMap, publicKey)
		} else {
			pm.PublicKeyToPeersMap[publicKey] = remaining
		}
	}

	for receiverID, peer := range pm.ReceiverToPeerMap {
		if now.Sub(peer.Timestamp) >= expire {
			pm.logger.Debug("Remove key from ReceiverToPeerMap: %x", receiverID)
			delete(pm.ReceiverToPeerMap, receiverID)
		}
	}

	return nil
}

func CalculateMac1Key(publicKey PublicKey) (Mac1Key, error) {
	var mac1Key Mac1Key
	hash, err := blake2s.New256(nil)
	if err != nil {
		return mac1Key, err
	}

	hash.Write([]byte(WGLabelMAC1))
	hash.Write(publicKey[:])
	hash.Sum(mac1Key[:0])

	return mac1Key, nil
}

func AppendUniqueValue[T comparable, V comparable](m map[T][]V, key T, value V, equal func(a, b V) bool) {
	for _, v := range m[key] {
		if equal(v, value) {
			return
		}
	}
	m[key] = append(m[key], value)
}
