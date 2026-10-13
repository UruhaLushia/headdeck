package service

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"sync"
	"time"

	"github.com/UruhaLushia/headdeck/internal/ipc/protocol"
)

type Operations struct {
	mu       sync.Mutex
	byID     map[string]*protocol.Operation
	order    []string
	max      int
	now      func() time.Time
	nextID   func() string
	changed  chan struct{}
	onChange []func(*protocol.Operation)
}

func NewOperations() *Operations {
	return &Operations{
		byID:    map[string]*protocol.Operation{},
		max:     128,
		now:     time.Now,
		nextID:  newOperationID,
		changed: make(chan struct{}, 1),
	}
}

func newOperationID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "op-" + hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))[:16]
	}
	return "op-" + hex.EncodeToString(b[:])
}

func (o *Operations) Start(kind, target, requestedBy string) *protocol.Operation {
	o.mu.Lock()
	defer o.mu.Unlock()
	op := &protocol.Operation{
		ID:          o.nextID(),
		Kind:        kind,
		State:       protocol.OperationPending,
		Target:      target,
		RequestedBy: requestedBy,
		StartedAt:   o.now().UTC().Format(time.RFC3339),
	}
	o.byID[op.ID] = op
	o.order = append(o.order, op.ID)
	for len(o.order) > o.max {
		delete(o.byID, o.order[0])
		remaining := copy(o.order, o.order[1:])
		o.order[remaining] = ""
		o.order = o.order[:remaining]
	}
	o.notifyLocked()
	return cloneOperation(op)
}

func (o *Operations) Get(id string) (*protocol.Operation, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	op, ok := o.byID[id]
	if !ok {
		return nil, false
	}
	return cloneOperation(op), true
}

func (o *Operations) List() []*protocol.Operation {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]*protocol.Operation, 0, len(o.order))
	for _, id := range o.order {
		if op, ok := o.byID[id]; ok {
			out = append(out, cloneOperation(op))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt > out[j].StartedAt })
	return out
}

func (o *Operations) Update(id string, fn func(*protocol.Operation)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	op, ok := o.byID[id]
	if !ok {
		return
	}
	fn(op)
	o.notifyLocked()
	for _, listener := range o.onChange {
		listener(cloneOperation(op))
	}
}

func (o *Operations) SetState(id string, state protocol.OperationState, message string) {
	o.Update(id, func(op *protocol.Operation) {
		op.State = state
		if message != "" {
			op.Message = message
		}
		if state == protocol.OperationSucceeded || state == protocol.OperationFailed ||
			state == protocol.OperationTimeout {
			op.FinishedAt = o.now().UTC().Format(time.RFC3339)
		}
	})
}

func (o *Operations) AddStep(id string, step protocol.OperationStep) {
	if step.At == "" {
		step.At = o.now().UTC().Format(time.RFC3339)
	}
	o.Update(id, func(op *protocol.Operation) {
		op.Steps = append(op.Steps, step)
		if step.State == protocol.OperationRunning {
			op.State = protocol.OperationRunning
		}
	})
}

func (o *Operations) Fail(id string, err *protocol.Error, message string) {
	o.Update(id, func(op *protocol.Operation) {
		op.State = protocol.OperationFailed
		op.Error = err
		if message != "" {
			op.Message = message
		}
		op.FinishedAt = o.now().UTC().Format(time.RFC3339)
	})
}

func (o *Operations) Changed() <-chan struct{} { return o.changed }

// Listeners run under the registry lock, so they must not call back into it.
func (o *Operations) OnChange(fn func(*protocol.Operation)) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.onChange = append(o.onChange, fn)
}

func (o *Operations) notifyLocked() {
	select {
	case o.changed <- struct{}{}:
	default:
	}
}

func cloneOperation(op *protocol.Operation) *protocol.Operation {
	if op == nil {
		return nil
	}
	cp := *op
	if len(op.Steps) > 0 {
		cp.Steps = append([]protocol.OperationStep(nil), op.Steps...)
	}
	return &cp
}
