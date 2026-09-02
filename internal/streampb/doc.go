// Package streampb holds the generated protobuf bindings for Webull's MQTT
// market-data payloads.
//
// message.proto is vendored verbatim from Webull's official SDK repositories
// (github.com/webull-inc/webull-openapi-python-sdk, Apache License 2.0),
// where it defines the payload for each streaming topic. It is pinned here
// so the SDK builds against a known schema rather than upstream drift.
//
// Regenerate with:
//
//	protoc --go_out=. --go_opt=paths=source_relative \
//	  --go_opt=Minternal/streampb/message.proto=github.com/sfreiberg/webull/internal/streampb \
//	  internal/streampb/message.proto
//
// The generated code is committed so building the SDK needs no protobuf
// toolchain.
package streampb
