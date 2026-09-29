# tug

Live **vertical scaling** of container resources for the [c12s](https://github.com/c12s)
distributed edge-cloud platform. `tug` changes a running container's CPU, memory and
block-IO limits at runtime, without a restart, through a **pluggable runtime engine**
(Docker now).




## Install / build

Requires Go 1.26+ and a running Docker daemon.

```bash
cd tug
go mod tidy      
go build -o tug ./cmd/tug
go test ./...    # unit tests run without Docker (fake engine)
```

## CLI usage

```bash
tug inspect --container web
tug set     --container web --cpu 1.5 --memory 512MiB
tug add     --container web --cpu 0.5
tug remove  --container web --memory 128MiB
tug set     --container web --cpu 1 --dry-run   # preview, no change made
```

## Architecture 

```
CLI / Go API  -  Scaler  -  RuntimeEngine (interface)  -  Docker                                                          
```

- **`pkg/resource`** - runtime-agnostic `ResourceSet` + `Patch` + arithmetic/validation.
- **`pkg/engine`** - the `RuntimeEngine` interface + a name registry.
- **`pkg/engine/docker`** - Docker implementation (`ContainerUpdate`).
- **`pkg/scaler`** - Core.

In c12s, tug is used as a library by the `star` node agent, which receives meridian's
commands over NATS and calls the scaler.

## Swapping the runtime engine

Implement `engine.RuntimeEngine` (five methods: `Inspect`, `Update`, `Remove`,
`Capabilities`, `Close`) and call `engine.Register("containerd", factory)` in an `init()`.
Import the new package in `cmd/tug/main.go` the same way the Docker engine is imported, then
select it with `--engine containerd`.

## Note on `disk`

The model's third shareable resource is disk. Docker **cannot change** disk *capacity* live
(`--storage-opt size` is create-time and driver-specific), so `tug` enforces disk **I/O**
(`BlkioWeight`) live and reports disk *capacity* as an unsupported live capability for the
Docker engine. Another engine may support it.

## Demo

```bash
bash examples/demo.sh
```

Starts an nginx container, scales it live, and prints before/after limits from `docker inspect`
to show the change took effect without a restart.
