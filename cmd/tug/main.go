// command-line interface for live vertical scaling of container resources
//
//	tug inspect --container web
//	tug set     --container web --cpu 1.5 --memory 512MiB
//	tug add     --container web --cpu 0.5
//	tug remove  --container web --memory 128MiB
//	tug set     --container web --cpu 1 --dry-run
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/c12s/tug/pkg/engine"
	"github.com/c12s/tug/pkg/resource"
	"github.com/c12s/tug/pkg/scaler"

	_ "github.com/c12s/tug/pkg/engine/docker"

	units "github.com/docker/go-units"
)

const usage = `tug - live vertical scaling of container resources

Usage:
  tug <command> [flags]

Commands:
  inspect   Show a container's current resource limits
  set       Set resources to absolute values
  add       Increase resources by a delta
  remove    Decrease resources by a delta
  version   Print version
  help      Show this help

Common flags:
  --container string   container id or name (required)
  --engine string      runtime engine (default "docker")
  --timeout duration   operation timeout (default 15s)

Resource flags (set/add/remove):
  --cpu float          number of cores, e.g. 1.5
  --memory string      memory limit, e.g. 512MiB, 1g
  --memory-swap string memory+swap limit
  --memory-reservation string  soft memory limit
  --io-weight uint     block-IO weight (10..1000)
  --dry-run            compute and print without applying
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	var err error
	switch cmd {
	case "inspect":
		err = runInspect(args)
	case "set":
		err = runScale(scaler.OpSet, args)
	case "add":
		err = runScale(scaler.OpAdd, args)
	case "remove":
		err = runScale(scaler.OpRemove, args)
	case "version":
		fmt.Println("tug", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

var version = "0.1.0-dev"

func runInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ExitOnError)
	engineName := fs.String("engine", "docker", "runtime engine")
	ref := fs.String("container", "", "container id or name (required)")
	timeout := fs.Duration("timeout", 15*time.Second, "operation timeout")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ref == "" {
		return fmt.Errorf("--container is required")
	}

	eng, err := engine.New(*engineName, nil)
	if err != nil {
		return err
	}
	defer eng.Close()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	cur, err := eng.Inspect(ctx, *ref)
	if err != nil {
		return err
	}
	fmt.Printf("container %s (engine %s)\n", *ref, eng.Capabilities().Name)
	printSet("  current", cur)
	return nil
}

func runScale(op scaler.Op, args []string) error {
	fs := flag.NewFlagSet(string(op), flag.ExitOnError)
	engineName := fs.String("engine", "docker", "runtime engine")
	ref := fs.String("container", "", "container id or name (required)")
	timeout := fs.Duration("timeout", 15*time.Second, "operation timeout")
	dryRun := fs.Bool("dry-run", false, "compute and print without applying")

	cpu := fs.Float64("cpu", 0, "number of cores")
	mem := fs.String("memory", "", "memory limit (e.g. 512MiB)")
	memSwap := fs.String("memory-swap", "", "memory+swap limit")
	memRes := fs.String("memory-reservation", "", "soft memory limit")
	ioWeight := fs.Uint("io-weight", 0, "block-IO weight (10-1000)")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if *ref == "" {
		return fmt.Errorf("--container is required")
	}

	// track which flags the user actually set
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })

	patch, err := buildPatch(set, *cpu, *mem, *memSwap, *memRes, *ioWeight)
	if err != nil {
		return err
	}

	eng, err := engine.New(*engineName, nil)
	if err != nil {
		return err
	}
	defer eng.Close()

	s := scaler.New(eng)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	res, err := s.Apply(ctx, scaler.Request{
		Ref:    *ref,
		Op:     op,
		Patch:  patch,
		DryRun: *dryRun,
	})
	if err != nil {
		return err
	}

	tag := "applied"
	if res.DryRun {
		tag = "dry-run (not applied)"
	}
	fmt.Printf("%s %s on %s [%s]\n", op, "resources", res.Ref, tag)
	printSet("  before", res.Previous)
	printSet("  after ", res.Applied)
	return nil
}

func buildPatch(set map[string]bool, cpu float64, mem, memSwap, memRes string, ioWeight uint) (resource.Patch, error) {
	var p resource.Patch
	if set["cpu"] {
		v := cpu
		p.CPU = &v
	}
	if set["memory"] {
		b, err := units.RAMInBytes(mem)
		if err != nil {
			return p, fmt.Errorf("--memory: %w", err)
		}
		p.Memory = &b
	}
	if set["memory-swap"] {
		b, err := units.RAMInBytes(memSwap)
		if err != nil {
			return p, fmt.Errorf("--memory-swap: %w", err)
		}
		p.MemorySwap = &b
	}
	if set["memory-reservation"] {
		b, err := units.RAMInBytes(memRes)
		if err != nil {
			return p, fmt.Errorf("--memory-reservation: %w", err)
		}
		p.MemoryReservation = &b
	}
	if set["io-weight"] {
		w := uint16(ioWeight)
		p.IOWeight = &w
	}
	if p.IsEmpty() {
		return p, fmt.Errorf("no resource flags given (use --cpu, --memory, ...)")
	}
	return p, nil
}

func printSet(label string, r resource.ResourceSet) {
	cpu := "unlimited"
	if r.CPU > 0 {
		cpu = fmt.Sprintf("%g cores", r.CPU)
	}
	mem := "unlimited"
	if r.Memory > 0 {
		mem = units.BytesSize(float64(r.Memory))
	}
	io := "unset"
	if r.IOWeight > 0 {
		io = fmt.Sprintf("%d", r.IOWeight)
	}
	fmt.Printf("%s: cpu=%s memory=%s io-weight=%s\n", label, cpu, mem, io)
}
