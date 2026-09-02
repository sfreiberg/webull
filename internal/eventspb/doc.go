// Package eventspb holds the generated protobuf and gRPC bindings for
// Webull's trade event stream.
//
// events.proto is vendored verbatim from Webull's official SDK repositories
// (github.com/webull-inc/webull-openapi-python-sdk, Apache License 2.0),
// where it defines the grpc.trade.event.EventService. It is pinned here so
// the SDK builds against a known schema rather than upstream drift.
//
// Regenerate with:
//
//	protoc --go_out=. --go_opt=paths=source_relative \
//	  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
//	  --go_opt=Minternal/eventspb/events.proto=github.com/sfreiberg/webull/internal/eventspb \
//	  --go-grpc_opt=Minternal/eventspb/events.proto=github.com/sfreiberg/webull/internal/eventspb \
//	  internal/eventspb/events.proto
//
// The generated code is committed so building the SDK needs no protobuf
// toolchain.
package eventspb
