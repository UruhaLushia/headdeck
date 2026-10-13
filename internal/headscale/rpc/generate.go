// Package rpc contains the wire messages used by the Headscale Unix client.
package rpc

//go:generate protoc --go_out=. --go_opt=paths=source_relative management.proto
