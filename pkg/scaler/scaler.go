// runtime core of tug
//turns a scaling Request (set, add, remove) into an absolute target, validates and clamps it and applies it through a RuntimeEngine
package scaler

import (
	"context"
	"fmt"
	"sync"

	"github.com/c12s/tug/pkg/engine"
	"github.com/c12s/tug/pkg/resource"
)

// scaling operation
type Op string

const (
	OpSet    Op = "set"   
	OpAdd    Op = "add"   
	OpRemove Op = "remove"
)

// single command for container
type Request struct {
	Ref    string
	Op     Op
	Patch  resource.Patch
	DryRun bool
}

// reports the before/after values of an operation
type Result struct {
	Ref      string
	Op       Op
	Previous resource.ResourceSet
	Applied  resource.ResourceSet
	DryRun   bool
}

// applies scaling requests through a runtime engine
type Scaler struct {
	engine engine.RuntimeEngine
	limits Limits
	locks  keyedMutex
}

// configures a Scaler
type Option func(*Scaler)

// sets the safety bounds (defaul DefaultLimits)
func WithLimits(l Limits) Option { return func(s *Scaler) { s.limits = l } }

// creates a Scaler over the given engine
func New(e engine.RuntimeEngine, opts ...Option) *Scaler {
	s := &Scaler{
		engine: e,
		limits: DefaultLimits(),
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// reports what the underlying engine can change at runtime
func (s *Scaler) Capabilities() engine.Capabilities {
	return s.engine.Capabilities()
}

// executes a scaling request and returns the before/after values
func (s *Scaler) Apply(ctx context.Context, req Request) (Result, error) {
	if req.Ref == "" {
		return Result{}, fmt.Errorf("scaler: empty container ref")
	}
	if req.Patch.IsEmpty() {
		return Result{}, fmt.Errorf("scaler: no resources specified")
	}

	unlock := s.locks.lock(req.Ref)
	defer unlock()

	current, err := s.engine.Inspect(ctx, req.Ref)
	if err != nil {
		return Result{}, err
	}

	target, err := computeTarget(current, req)
	if err != nil {
		return Result{}, err
	}

	if err := s.checkCapabilities(current, target); err != nil {
		return Result{}, err
	}
	target = s.limits.Clamp(target)
	target = followSwap(current, target, req.Patch.MemorySwap != nil)
	if err := target.Validate(); err != nil {
		return Result{}, fmt.Errorf("scaler: invalid target: %w", err)
	}
	if err := s.limits.Check(target); err != nil {
		return Result{}, err
	}

	res := Result{
		Ref:      req.Ref,
		Op:       req.Op,
		Previous: current,
		Applied:  target,
		DryRun:   req.DryRun,
	}

	if req.DryRun {
		return res, nil
	}

	if err := s.engine.Update(ctx, req.Ref, target); err != nil {
		return Result{}, err
	}
	return res, nil
}

// stops and deletes the container
// callers holding a Scaler can tear down a container through the same runtime engine
func (s *Scaler) RemoveContainer(ctx context.Context, ref string) error {
	unlock := s.locks.lock(ref)
	defer unlock()
	return s.engine.Remove(ctx, ref)
}

// followSwap keeps the containers swap allowance when the memory limit changes 
func followSwap(current, target resource.ResourceSet, swapRequested bool) resource.ResourceSet {
	if swapRequested || target.Memory == current.Memory {
		return target
	}

	if current.Memory <= 0 || current.MemorySwap <= 0 || target.Memory <= 0 {
		return target
	}
	allowance := current.MemorySwap - current.Memory
	if allowance < 0 {
		allowance = 0
	}
	target.MemorySwap = target.Memory + allowance
	return target
}

func computeTarget(current resource.ResourceSet, req Request) (resource.ResourceSet, error) {
	switch req.Op {
	case OpSet:
		return current.ApplySet(req.Patch), nil
	case OpAdd:
		return current.ApplyAdd(req.Patch), nil
	case OpRemove:
		return current.ApplyRemove(req.Patch), nil
	default:
		return resource.ResourceSet{}, fmt.Errorf("scaler: unknown op %q", req.Op)
	}
}

// rejects requests that would change a resource the engine cannot do live
func (s *Scaler) checkCapabilities(current, target resource.ResourceSet) error {
	c := s.engine.Capabilities()
	if !c.CPU && target.CPU != current.CPU {
		return fmt.Errorf("scaler: engine %q cannot change cpu at runtime", c.Name)
	}
	if !c.Memory && (target.Memory != current.Memory ||
		target.MemorySwap != current.MemorySwap ||
		target.MemoryReservation != current.MemoryReservation) {
		return fmt.Errorf("scaler: engine %q cannot change memory at runtime", c.Name)
	}
	if !c.IO && target.IOWeight != current.IOWeight {
		return fmt.Errorf("scaler: engine %q cannot change io weight at runtime", c.Name)
	}
	return nil
}

// provides one mutex per string key
type keyedMutex struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func (k *keyedMutex) lock(key string) func() {
	k.mu.Lock()
	if k.m == nil {
		k.m = make(map[string]*sync.Mutex)
	}
	mu, ok := k.m[key]
	if !ok {
		mu = &sync.Mutex{}
		k.m[key] = mu
	}
	k.mu.Unlock()

	mu.Lock()
	return mu.Unlock
}
