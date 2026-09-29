package scaler

import (
	"context"
	"testing"

	"github.com/c12s/tug/pkg/engine"
	"github.com/c12s/tug/pkg/resource"
)

// in memory RuntimeEngine for tests
type fakeEngine struct {
	state map[string]resource.ResourceSet
	caps  engine.Capabilities
	updates int
}

func newFakeEngine() *fakeEngine {
	return &fakeEngine{
		state: map[string]resource.ResourceSet{},
		caps:  engine.Capabilities{Name: "fake", CPU: true, Memory: true, IO: true},
	}
}

func (f *fakeEngine) Inspect(_ context.Context, ref string) (resource.ResourceSet, error) {
	return f.state[ref], nil
}

func (f *fakeEngine) Update(_ context.Context, ref string, t resource.ResourceSet) error {
	f.updates++
	f.state[ref] = t
	return nil
}

func (f *fakeEngine) Remove(_ context.Context, ref string) error {
	delete(f.state, ref)
	return nil
}
func (f *fakeEngine) Capabilities() engine.Capabilities { return f.caps }
func (f *fakeEngine) Close() error                      { return nil }

func f64(v float64) *float64 { return &v }
func i64(v int64) *int64     { return &v }

func TestSetOverlaysOnlyGivenFields(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{CPU: 1, Memory: 256 << 20}
	s := New(fe)

	res, err := s.Apply(context.Background(), Request{
		Ref:   "web",
		Op:    OpSet,
		Patch: resource.Patch{CPU: f64(2)}, // memory untouched
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied.CPU != 2 {
		t.Errorf("cpu = %g, want 2", res.Applied.CPU)
	}
	if res.Applied.Memory != 256<<20 {
		t.Errorf("memory = %d, want unchanged", res.Applied.Memory)
	}
	if res.Previous.CPU != 1 {
		t.Errorf("previous cpu = %g, want 1", res.Previous.CPU)
	}
}

func TestAddAndRemove(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{CPU: 1, Memory: 512 << 20}
	s := New(fe)

	add, err := s.Apply(context.Background(), Request{
		Ref: "web", Op: OpAdd, Patch: resource.Patch{CPU: f64(0.5)},
	})
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if add.Applied.CPU != 1.5 {
		t.Errorf("after add cpu = %g, want 1.5", add.Applied.CPU)
	}

	rm, err := s.Apply(context.Background(), Request{
		Ref: "web", Op: OpRemove, Patch: resource.Patch{Memory: i64(256 << 20)},
	})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if rm.Applied.Memory != 256<<20 {
		t.Errorf("after remove memory = %d, want 256MiB", rm.Applied.Memory)
	}
}

func TestRemoveClampedToMinMemory(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{Memory: 8 << 20} // 8MiB
	s := New(fe) // DefaultLimits: MinMemory 6MiB

	res, err := s.Apply(context.Background(), Request{
		Ref: "web", Op: OpRemove, Patch: resource.Patch{Memory: i64(7 << 20)},
	})
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if res.Applied.Memory != 6<<20 {
		t.Errorf("memory = %d, want clamped to 6MiB", res.Applied.Memory)
	}
}

func TestDryRunDoesNotApply(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{CPU: 1}
	s := New(fe)

	_, err := s.Apply(context.Background(), Request{
		Ref: "web", Op: OpSet, Patch: resource.Patch{CPU: f64(4)}, DryRun: true,
	})
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if fe.updates != 0 {
		t.Errorf("engine.Update called %d times during dry-run, want 0", fe.updates)
	}
	if fe.state["web"].CPU != 1 {
		t.Errorf("state changed during dry-run: cpu = %g", fe.state["web"].CPU)
	}
}

func TestCapabilityRejection(t *testing.T) {
	fe := newFakeEngine()
	fe.caps.CPU = false // engine cannot change cpu live
	fe.state["web"] = resource.ResourceSet{CPU: 1}
	s := New(fe)

	_, err := s.Apply(context.Background(), Request{
		Ref: "web", Op: OpSet, Patch: resource.Patch{CPU: f64(2)},
	})
	if err == nil {
		t.Fatal("expected error when engine cannot change cpu, got nil")
	}
}

func TestMaxCPUEnforced(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{CPU: 1}
	s := New(fe, WithLimits(Limits{MaxCPU: 2, MinMemory: 6 << 20}))

	_, err := s.Apply(context.Background(), Request{
		Ref: "web", Op: OpSet, Patch: resource.Patch{CPU: f64(8)},
	})
	if err == nil {
		t.Fatal("expected max-cpu error, got nil")
	}
}

// docker sets memory+swap to 2x memory at create time
//growing memory past that must move swap along (keeping the swap allowance)
func TestMemoryGrowthFollowsSwap(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{Memory: 512 << 20, MemorySwap: 1024 << 20} // allowance 512MiB
	s := New(fe)

	res, err := s.Apply(context.Background(), Request{Ref: "web", Op: OpSet, Patch: resource.Patch{Memory: i64(2048 << 20)}})
	if err != nil {
		t.Fatalf("growing memory past the old swap value should succeed: %v", err)
	}
	if res.Applied.MemorySwap != 2560<<20 {
		t.Errorf("swap = %d MiB, want 2560 MiB (2048 + 512 allowance)", res.Applied.MemorySwap>>20)
	}
}

// shrinking memory must shrink swap too, or the removed memory becomes swap
func TestMemoryShrinkFollowsSwap(t *testing.T) {
	fe := newFakeEngine()
	fe.state["web"] = resource.ResourceSet{Memory: 2048 << 20, MemorySwap: 4096 << 20}
	s := New(fe)

	res, err := s.Apply(context.Background(), Request{Ref: "web", Op: OpSet, Patch: resource.Patch{Memory: i64(1024 << 20)}})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied.MemorySwap != 3072<<20 {
		t.Errorf("swap = %d MiB, want 3072 MiB (1024 + 2048 allowance)", res.Applied.MemorySwap>>20)
	}
}

// an explicit swap value and unlimited swap (-1) are left as requested
func TestSwapLeftAloneWhenSetOrUnlimited(t *testing.T) {
	fe := newFakeEngine()
	fe.state["a"] = resource.ResourceSet{Memory: 512 << 20, MemorySwap: 1024 << 20}
	fe.state["b"] = resource.ResourceSet{Memory: 512 << 20, MemorySwap: -1}
	s := New(fe)
	ctx := context.Background()

	res, err := s.Apply(ctx, Request{Ref: "a", Op: OpSet, Patch: resource.Patch{Memory: i64(768 << 20), MemorySwap: i64(768 << 20)}})
	if err != nil || res.Applied.MemorySwap != 768<<20 {
		t.Errorf("explicit swap not kept: swap=%d err=%v", res.Applied.MemorySwap, err)
	}
	res, err = s.Apply(ctx, Request{Ref: "b", Op: OpSet, Patch: resource.Patch{Memory: i64(2048 << 20)}})
	if err != nil || res.Applied.MemorySwap != -1 {
		t.Errorf("unlimited swap not kept: swap=%d err=%v", res.Applied.MemorySwap, err)
	}
}
