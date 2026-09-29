// defines the pluggable runtime boundary of tug

package engine

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/c12s/tug/pkg/resource"
)

// makes the runtime replaceable.
type RuntimeEngine interface {
  
	// returns the containers current resource limits
	Inspect(ctx context.Context, ref string) (resource.ResourceSet, error)
  
	// applies resource limits to a running container
	Update(ctx context.Context, ref string, target resource.ResourceSet) error
  
	//stops and deletes the container
	Remove(ctx context.Context, ref string) error
  
	// finds which resources this engine can change at runtime
	Capabilities() Capabilities
  
	// releases any client/connection
	Close() error
}

// describes what a given engine can enforce live
// used to reject requests an engine cannot honor (disk capacity on Docker)
type Capabilities struct {
	Name         string
	CPU          bool
	Memory       bool
	IO           bool
	DiskCapacity bool 
}

// builds an engine from an options map 
type Factory func(opts map[string]string) (RuntimeEngine, error)

var (
	regMu    sync.RWMutex
	registry = map[string]Factory{}
)

//makes an engine available under a name
//implementations call this from init()
func Register(name string, f Factory) {
	regMu.Lock()
	defer regMu.Unlock()
	registry[name] = f
}

// constructs a registered engine by name
func New(name string, opts map[string]string) (RuntimeEngine, error) {
	regMu.RLock()
	f, ok := registry[name]
	regMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown runtime engine %q (available: %v)", name, Registered())
	}
	return f(opts)
}

// lists the names of all registered engines
func Registered() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
