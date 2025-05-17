package main

import (
	"context"
	"net"
	"sync"
)

type PacketJob struct {
	Addr *net.UDPAddr
	Data []byte
}

type WorkerPool struct {
	jobQueue   chan PacketJob
	wg         sync.WaitGroup
	maxWorkers int
	logger     LoggerInterface
	handler    func(context.Context, *net.UDPAddr, []byte) error
}

func NewWorkerPool(maxWorkers int, handler func(context.Context, *net.UDPAddr, []byte) error, logger LoggerInterface) *WorkerPool {
	if maxWorkers < 1 {
		maxWorkers = 1
	}

	return &WorkerPool{
		jobQueue:   make(chan PacketJob, maxWorkers*2),
		maxWorkers: maxWorkers,
		logger:     logger,
		handler:    handler,
	}
}

func (wp *WorkerPool) Start(ctx context.Context) {
	wp.logger.Info("Starting worker pool with %d workers", wp.maxWorkers)

	for i := 0; i < wp.maxWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(ctx, i)
	}
}

func (wp *WorkerPool) worker(ctx context.Context, id int) {
	defer wp.wg.Done()

	wp.logger.Debug("Worker %d started", id)

	for {
		select {
		case <-ctx.Done():
			wp.logger.Debug("Worker %d shutting down", id)
			return
		case job, ok := <-wp.jobQueue:
			if !ok {
				wp.logger.Debug("Worker %d: job queue closed", id)
				return
			}

			err := wp.handler(ctx, job.Addr, job.Data)
			if err != nil {
				wp.logger.Error("Worker %d: failed to handle packet: %v", id, err)
			}
		}
	}
}

func (wp *WorkerPool) Submit(addr *net.UDPAddr, data []byte) bool {
	job := PacketJob{
		Addr: addr,
		Data: data,
	}
	
	select {
	case wp.jobQueue <- job:
		return true
	default:
		return false
	}
}

func (wp *WorkerPool) Shutdown() {
	close(wp.jobQueue)
	wp.wg.Wait()
	wp.logger.Info("Worker pool shutdown complete")
}
