// Package securechannel contains the OpenTunnel v1 secure-channel spike.
//
// This package is intentionally narrow. It proves the Noise handshake,
// prologue binding, PSK handling, host key verification, and encrypted frame
// behavior required before relay or command-execution work begins.
//
// The package must not import relay, CLI, command runner, or artifact-serving
// code. Higher-level product code should depend on this package through small
// functions and data types rather than knowing Noise library details.
package securechannel
