package stt

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
)

// One request carries at most a 30-second recognition window, with ample JSON
// overhead. Framing bounds allocation before decoding untrusted worker bytes.
const engineFrameLimit = 16 << 20

type engineRequest struct {
	Op      string
	Lease   LeaseOptions
	PCM     []float32
	Options Options
}
type engineResponse struct {
	Error      string
	OverBudget bool
	Models     []ModelInfo
	Info       *ModelInfo
	Result     *Result
}

func engineRead(c net.Conn, value any) error {
	var header [4]byte
	if _, err := io.ReadFull(c, header[:]); err != nil {
		return err
	}
	n := binary.BigEndian.Uint32(header[:])
	if n > engineFrameLimit {
		return fmt.Errorf("speech request exceeds frame limit")
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(c, data); err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}
func engineWrite(c net.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > engineFrameLimit {
		return fmt.Errorf("speech response exceeds frame limit")
	}
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], uint32(len(data)))
	if _, err = c.Write(header[:]); err != nil {
		return err
	}
	_, err = c.Write(data)
	return err
}

// ServeEngine exposes the node's one scheduler to media-isolated workers. Each
// lease belongs to its connection; EOF cancels native inference and releases it.
func ServeEngine(ctx context.Context, path string, engine Engine) (func(), error) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0600); err != nil {
		listener.Close()
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); listener.Close(); os.Remove(path) }) }
	go func() { <-ctx.Done(); stop() }()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go serveEngineConnection(ctx, conn, engine)
		}
	}()
	return stop, nil
}
func serveEngineConnection(parent context.Context, c net.Conn, engine Engine) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	defer c.Close()
	requests := make(chan engineRequest)
	go func() {
		defer cancel()
		defer close(requests)
		for {
			var r engineRequest
			if engineRead(c, &r) != nil {
				return
			}
			select {
			case requests <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() { <-ctx.Done(); c.Close() }()
	var lease Lease
	defer func() {
		if lease != nil {
			lease.Release()
		}
	}()
	for r := range requests {
		var response engineResponse
		var err error
		switch r.Op {
		case "models":
			if engine != nil {
				response.Models = engine.Models()
			}
		case "lease":
			if lease != nil {
				err = fmt.Errorf("connection already holds a lease")
			} else if engine == nil {
				err = ErrOverBudget
			} else {
				lease, err = engine.Lease(ctx, r.Lease)
			}
		case "model", "transcribe":
			if lease == nil {
				err = fmt.Errorf("speech lease required")
				break
			}
			model := lease.Model()
			if model == nil {
				err = ErrOverBudget
				break
			}
			info := model.Info()
			response.Info = &info
			if r.Op == "transcribe" {
				if len(r.PCM) > 30*SampleRate {
					err = fmt.Errorf("speech window exceeds 30 seconds")
				} else {
					response.Result, err = model.Transcribe(ctx, r.PCM, r.Options)
				}
			}
		default:
			err = fmt.Errorf("unknown speech operation")
		}
		if err != nil {
			response.Error = err.Error()
			response.OverBudget = err == ErrOverBudget
		}
		if engineWrite(c, response) != nil {
			return
		}
	}
}

type proxyEngine struct {
	path        string
	mu          sync.Mutex
	closed      bool
	connections map[*proxyLease]struct{}
}
type proxyLease struct {
	conn     net.Conn
	engine   *proxyEngine
	mu       sync.Mutex
	released bool
}
type proxyModel struct {
	lease *proxyLease
	info  ModelInfo
}

// NewProxy never loads a private model or substitutes an independent budget.
func NewProxy(path string) Engine {
	return &proxyEngine{path: path, connections: make(map[*proxyLease]struct{})}
}
func (e *proxyEngine) Close() error {
	e.mu.Lock()
	e.closed = true
	connections := e.connections
	e.connections = nil
	e.mu.Unlock()
	for lease := range connections {
		lease.Release()
	}
	return nil
}
func (e *proxyEngine) connect(ctx context.Context) (*proxyLease, error) {
	c, err := (&net.Dialer{}).DialContext(ctx, "unix", e.path)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		c.Close()
		return nil, fmt.Errorf("speech engine closed")
	}
	l := &proxyLease{conn: c, engine: e}
	e.connections[l] = struct{}{}
	return l, nil
}
func (e *proxyEngine) Models() []ModelInfo {
	ctx := context.Background()
	l, err := e.connect(ctx)
	if err != nil {
		return nil
	}
	defer l.Release()
	r, err := l.call(ctx, engineRequest{Op: "models"})
	if err != nil {
		return nil
	}
	return r.Models
}
func (e *proxyEngine) Lease(ctx context.Context, opts LeaseOptions) (Lease, error) {
	l, err := e.connect(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = l.call(ctx, engineRequest{Op: "lease", Lease: opts}); err != nil {
		l.Release()
		return nil, err
	}
	return l, nil
}
func (l *proxyLease) call(ctx context.Context, r engineRequest) (engineResponse, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	var response engineResponse
	if l.released {
		return response, fmt.Errorf("speech lease released")
	}
	stop := context.AfterFunc(ctx, func() { l.conn.Close() })
	defer stop()
	if err := engineWrite(l.conn, r); err != nil {
		return response, err
	}
	if err := engineRead(l.conn, &response); err != nil {
		if ctx.Err() != nil {
			return response, ctx.Err()
		}
		return response, err
	}
	if response.OverBudget {
		return response, ErrOverBudget
	}
	if response.Error != "" {
		return response, fmt.Errorf("remote speech engine: %s", response.Error)
	}
	return response, nil
}
func (l *proxyLease) Release() {
	l.conn.Close()
	l.mu.Lock()
	l.released = true
	l.mu.Unlock()
	if l.engine != nil {
		l.engine.mu.Lock()
		delete(l.engine.connections, l)
		l.engine.mu.Unlock()
	}
}
func (l *proxyLease) Model() Model {
	r, err := l.call(context.Background(), engineRequest{Op: "model"})
	if err != nil || r.Info == nil {
		return nil
	}
	return &proxyModel{lease: l, info: *r.Info}
}
func (m *proxyModel) Info() ModelInfo { return m.info }
func (m *proxyModel) Transcribe(ctx context.Context, pcm []float32, opts Options) (*Result, error) {
	r, err := m.lease.call(ctx, engineRequest{Op: "transcribe", PCM: pcm, Options: opts})
	return r.Result, err
}
