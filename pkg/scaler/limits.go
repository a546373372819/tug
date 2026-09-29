package scaler

import (
	"errors"
	"fmt"

	"github.com/c12s/tug/pkg/resource"
)

// safety bounds applied to every target
type Limits struct {
	MinCPU    float64 // cores, 0 allows unlimited
	MaxCPU    float64 // 0 = no cap
	MinMemory int64   // bytes, dockers floor is 6MiB
	MaxMemory int64   // 0 = no cap
}

// returns defaults
func DefaultLimits() Limits {
	return Limits{
		MinMemory: 6 * 1024 * 1024,
	}
}

// raises each in use field to its configured minimum
func (l Limits) Clamp(r resource.ResourceSet) resource.ResourceSet {
	if r.CPU != 0 && l.MinCPU > 0 && r.CPU < l.MinCPU {
		r.CPU = l.MinCPU
	}
	if r.Memory != 0 && l.MinMemory > 0 && r.Memory < l.MinMemory {
		r.Memory = l.MinMemory
	}
	return r
}

// verifies target hard caps that must not be clamped
func (l Limits) Check(r resource.ResourceSet) error {
	var errs []error
	if l.MaxCPU > 0 && r.CPU > l.MaxCPU {
		errs = append(errs, fmt.Errorf("cpu %g exceeds max %g", r.CPU, l.MaxCPU))
	}
	if l.MaxMemory > 0 && r.Memory > l.MaxMemory {
		errs = append(errs, fmt.Errorf("memory %d exceeds max %d", r.Memory, l.MaxMemory))
	}
	return errors.Join(errs...)
}
