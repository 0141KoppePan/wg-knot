package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func setupSignalHandler(ctx context.Context, cancel context.CancelFunc, logger LoggerInterface) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigCh:
			logger.Info("Received signal: %v, initiating graceful shutdown", sig)
			cancel()
		case <-ctx.Done():
			return
		}
	}()
}

func main() {
	fmt.Printf("WG Knot v%s\n", Version)

	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	logger := NewLogger(GetLogLevel(config.Server.LogLevel))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publicKeyPairList, err := LoadPublicKeyPairsFromConfig(config.KeyPairs)
	if err != nil {
		logger.Warning("Some public keys are invalid: %v", err)
	}

	if len(publicKeyPairList) == 0 {
		logger.Error("No valid public key pairs configured")
		os.Exit(1)
	}

	addr, err := net.ResolveUDPAddr("udp",
		net.JoinHostPort(config.Server.ListenAddress,
			strconv.Itoa(config.Server.Port)))
	if err != nil {
		logger.Error("Failed to resolve address: %v", err)
		os.Exit(1)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.Error("Failed to start UDP listener: %v", err)
		os.Exit(1)
	}
	defer conn.Close()

	err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		logger.Error("Failed to set read deadline: %v", err)
		os.Exit(1)
	}

	packetSender := NewUDPPacketSender(conn, logger)
	pm := NewPeerManager(packetSender, publicKeyPairList, logger, config.Server.PeerExpiration)

	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := pm.CleanupPeers(); err != nil {
				logger.Error("Failed to cleanup peers: %v", err)
			}
		}
	}()

	bufferPool := NewBufferPool(config.BufferPool.PoolSize, config.BufferPool.BufferSize)
	logger.Info("Buffer pool created: size=%d, buffer size=%d bytes",
		config.BufferPool.PoolSize, config.BufferPool.BufferSize)

	workerPool := NewWorkerPool(config.WorkerPool.MaxWorkers, pm.HandlePacket, logger)
	workerPool.Start(ctx)
	logger.Info("Worker pool created: max workers=%d", config.WorkerPool.MaxWorkers)

	setupSignalHandler(ctx, cancel, logger)

	logger.Info("Started listening for UDP packets: %s:%d", config.Server.ListenAddress, config.Server.Port)

	for {
		select {
		case <-ctx.Done():
			logger.Info("Shutting down, waiting for worker pool to complete...")
			workerPool.Shutdown()
			logger.Info("Shutdown complete")
			return
		default:
			buffer := bufferPool.Get()

			err = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			if err != nil {
				logger.Error("Failed to set read deadline: %v", err)
				bufferPool.Put(buffer)
				continue
			}

			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					bufferPool.Put(buffer)
					continue
				}
				logger.Error("Packet reading error: %v", err)
				bufferPool.Put(buffer)
				continue
			}

			packetData := make([]byte, n)
			copy(packetData, buffer[:n])

			bufferPool.Put(buffer)

			if !workerPool.Submit(remoteAddr, packetData) {
				logger.Warning("Worker pool queue is full, packet dropped")
			}
		}
	}
}
