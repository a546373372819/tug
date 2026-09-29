// defines the runtime neutral resource model used by tug
// ResourceSet, Patch
// set, add, remove and validation used to calculate a scaling target.
// types here have no dependency on any container, runtime engines translate ResourceSet into their own API.
package resource

import (
	"errors"
	"fmt"
)

// set of resource limits for one container
// zero - unlimited / default
type ResourceSet struct {
	// number of cores
	CPU float64
	// memory limit in bytes
	Memory int64
	// memory + swap limit in bytes (must be >= Memory)
    // -1 unlimited
	MemorySwap int64
	// soft memory limit in bytes
	MemoryReservation int64
	//relative block-IO weight
    //valid range 10-1000
	IOWeight uint16
}

// partial change request
type Patch struct {
	CPU               *float64
	Memory            *int64
	MemorySwap        *int64
	MemoryReservation *int64
	IOWeight          *uint16
}

func (p Patch) IsEmpty() bool {
	return p.CPU == nil && p.Memory == nil && p.MemorySwap == nil &&
		p.MemoryReservation == nil && p.IOWeight == nil
}

// replaces the fields in the patch and returns the resulting absolute set
func (base ResourceSet) ApplySet(p Patch) ResourceSet {
	out := base
	if p.CPU != nil {
		out.CPU = *p.CPU
	}
	if p.Memory != nil {
		out.Memory = *p.Memory
	}
	if p.MemorySwap != nil {
		out.MemorySwap = *p.MemorySwap
	}
	if p.MemoryReservation != nil {
		out.MemoryReservation = *p.MemoryReservation
	}
	if p.IOWeight != nil {
		out.IOWeight = *p.IOWeight
	}
	return out
}

// adds the patch fields to base and returns the resulting absolute set
func (base ResourceSet) ApplyAdd(p Patch) ResourceSet {
	out := base
	if p.CPU != nil {
		out.CPU = base.CPU + *p.CPU
	}
	if p.Memory != nil {
		out.Memory = base.Memory + *p.Memory
	}
	if p.MemorySwap != nil {
		out.MemorySwap = base.MemorySwap + *p.MemorySwap
	}
	if p.MemoryReservation != nil {
		out.MemoryReservation = base.MemoryReservation + *p.MemoryReservation
	}
	if p.IOWeight != nil {
		out.IOWeight = addU16(base.IOWeight, *p.IOWeight)
	}
	return out
}

// subtracts the patch fields
// truncated subtraction so results never go negative and returns the result
func (base ResourceSet) ApplyRemove(p Patch) ResourceSet {
	out := base
	if p.CPU != nil {
		out.CPU = monusF(base.CPU, *p.CPU)
	}
	if p.Memory != nil {
		out.Memory = monusI(base.Memory, *p.Memory)
	}
	if p.MemorySwap != nil {
		out.MemorySwap = monusI(base.MemorySwap, *p.MemorySwap)
	}
	if p.MemoryReservation != nil {
		out.MemoryReservation = monusI(base.MemoryReservation, *p.MemoryReservation)
	}
	if p.IOWeight != nil {
		out.IOWeight = subU16(base.IOWeight, *p.IOWeight)
	}
	return out
}

func (r ResourceSet) Validate() error {
	var errs []error
	if r.CPU < 0 {
		errs = append(errs, fmt.Errorf("cpu must be >= 0, got %g", r.CPU))
	}
	if r.Memory < 0 {
		errs = append(errs, fmt.Errorf("memory must be >= 0, got %d", r.Memory))
	}
	if r.MemoryReservation < 0 {
		errs = append(errs, fmt.Errorf("memory reservation must be >= 0, got %d", r.MemoryReservation))
	}
  
	// swap must be >= memory when both are positive
	if r.Memory > 0 && r.MemorySwap > 0 && r.MemorySwap < r.Memory {
		errs = append(errs, fmt.Errorf(
			"memory swap (%d) must be >= memory (%d); set both together",
			r.MemorySwap, r.Memory))
	}
	if r.MemoryReservation > 0 && r.Memory > 0 && r.MemoryReservation > r.Memory {
		errs = append(errs, fmt.Errorf(
			"memory reservation (%d) must be <= memory (%d)",
			r.MemoryReservation, r.Memory))
	}
	if r.IOWeight != 0 && (r.IOWeight < 10 || r.IOWeight > 1000) {
		errs = append(errs, fmt.Errorf("io weight must be in [10,1000] or 0, got %d", r.IOWeight))
	}
	return errors.Join(errs...)
}


func monusF(a, b float64) float64 {
	if a-b < 0 {
		return 0
	}
	return a - b
}

func monusI(a, b int64) int64 {
	if a-b < 0 {
		return 0
	}
	return a - b
}

func addU16(a, b uint16) uint16 {
	sum := uint32(a) + uint32(b)
	if sum > 0xFFFF {
		return 0xFFFF
	}
	return uint16(sum)
}

func subU16(a, b uint16) uint16 {
	if b > a {
		return 0
	}
	return a - b
}
