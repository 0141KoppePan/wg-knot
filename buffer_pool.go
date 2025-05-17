package main

type BufferPool struct {
	pool       chan []byte
	bufferSize int
}

func NewBufferPool(poolSize int, bufferSize int) *BufferPool {
	return &BufferPool{
		pool:       make(chan []byte, poolSize),
		bufferSize: bufferSize,
	}
}

func (bp *BufferPool) Get() []byte {
	select {
	case buf := <-bp.pool:
		return buf
	default:
		return make([]byte, bp.bufferSize)
	}
}

func (bp *BufferPool) Put(buf []byte) {
	select {
	case bp.pool <- buf:
		// Return buffer to pool
	default:
		// Do nothing if the pool is full (buffer will be collected by GC)
	}
}
