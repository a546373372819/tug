module github.com/c12s/tug

go 1.26.0

require (
	github.com/docker/docker v27.3.1+incompatible
	github.com/docker/go-units v0.5.0
)

// PIN: docker v27.3.1 (used in +incompatible mode, so its own module
// requirements do not propagate) needs go-connections v0.5.0. Newer versions
// (>=0.8.1) removed sockets.DialPipe, which breaks the Windows build of the
// docker client. Do NOT `go get -u` this above v0.5.0 without re-verifying the
// build on Windows.
require github.com/docker/go-connections v0.5.0 // indirect

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/log v0.2.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/term v0.5.2 // indirect
	github.com/morikuni/aec v1.1.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.71.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	gotest.tools/v3 v3.5.2 // indirect
)
