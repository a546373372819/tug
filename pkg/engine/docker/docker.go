// docker implements engine.RuntimeEngine on top of the Docker Engine Go SDK using the ContainerUpdate API to change a running containers resources live

// self-registers under the name "docker"
// importing this package is enough to make the engine available 
//	import _ "github.com/c12s/tug/pkg/engine/docker"
package docker

import (
	"context"
	"fmt"

	"github.com/c12s/tug/pkg/engine"
	"github.com/c12s/tug/pkg/resource"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

// number of NanoCPUs in one core
const nanoPerCPU = 1_000_000_000

func init() {
	engine.Register("docker", func(opts map[string]string) (engine.RuntimeEngine, error) {
		return New(opts)
	})
}

// talks to a docker daemon
type Engine struct {
	cli *dockerclient.Client
}

// creates a docker engine
//Recognized opts:
//    "host": daemon host ("tcp://127.0.0.1:2375") 
//defaults to the environment (DOCKER_HOST / local socket) when empty
func New(opts map[string]string) (*Engine, error) {
	clientOpts := []dockerclient.Opt{
		dockerclient.FromEnv,
		dockerclient.WithAPIVersionNegotiation(),
	}
	if host := opts["host"]; host != "" {
		clientOpts = append(clientOpts, dockerclient.WithHost(host))
	}
	cli, err := dockerclient.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("docker: create client: %w", err)
	}
	return &Engine{cli: cli}, nil
}

//reads the containers current resource limits
func (e *Engine) Inspect(ctx context.Context, ref string) (resource.ResourceSet, error) {
	j, err := e.cli.ContainerInspect(ctx, ref)
	if err != nil {
		return resource.ResourceSet{}, fmt.Errorf("docker: inspect %q: %w", ref, err)
	}
	if j.HostConfig == nil {
		return resource.ResourceSet{}, fmt.Errorf("docker: inspect %q: no host config", ref)
	}
	r := j.HostConfig.Resources
	return resource.ResourceSet{
		CPU:               float64(r.NanoCPUs) / nanoPerCPU,
		Memory:            r.Memory,
		MemorySwap:        r.MemorySwap,
		MemoryReservation: r.MemoryReservation,
		IOWeight:          r.BlkioWeight,
	}, nil
}

// applies the absolute target to the running container.
func (e *Engine) Update(ctx context.Context, ref string, t resource.ResourceSet) error {
	res := container.Resources{
		NanoCPUs:          int64(t.CPU * nanoPerCPU),
		Memory:            t.Memory,
		MemoryReservation: t.MemoryReservation,
	}

	if t.MemorySwap != 0 {
		res.MemorySwap = t.MemorySwap
	}

	if t.IOWeight != 0 {
		res.BlkioWeight = t.IOWeight
	}

	_, err := e.cli.ContainerUpdate(ctx, ref, container.UpdateConfig{Resources: res})
	if err != nil {
		return fmt.Errorf("docker: update %q: %w", ref, err)
	}
	return nil
}

//forcestops and deletes the container.
func (e *Engine) Remove(ctx context.Context, ref string) error {
	if err := e.cli.ContainerRemove(ctx, ref, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("docker: remove %q: %w", ref, err)
	}
	return nil
}

func (e *Engine) Capabilities() engine.Capabilities {
	return engine.Capabilities{
		Name:         "docker",
		CPU:          true,
		Memory:       true,
		IO:           true,
		DiskCapacity: false,
	}
}

func (e *Engine) Close() error {
	return e.cli.Close()
}
