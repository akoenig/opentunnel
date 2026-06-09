# OpenTunnel Secure Channel Spike Decision

## Decision

OpenTunnel v1 will use `Noise_NKpsk0_25519_ChaChaPoly_BLAKE2s` for the first implementation milestone. The test suite in `internal/securechannel` passes against `github.com/flynn/noise`.

## Rationale

The client already receives the host session public key inside the opaque invite code, and the client has no durable identity in v1. `NKpsk0` maps directly to that model:

- client is anonymous,
- host has a per-session static key,
- client knows the host public key before the handshake,
- the invite's 32-byte `clientSecret` is mixed as the PSK,
- canonical OpenTunnel session context is bound through the prologue.

## Required Properties Verified By Tests

- 32-byte `clientSecret` PSK is accepted.
- Host session public key is verified against invite material.
- Prologue binding changes when session security context changes.
- Multiple encrypted frames can be exchanged after handshake.
- Both client-to-host and host-to-client transport directions are verified.
- Wrong `clientSecret` fails.
- Wrong `hostPubKey` fails.
- Wrong prologue fails.
- Replayed ciphertext fails.

## XXpsk3 Fallback Evaluation

`Noise_XXpsk3_25519_ChaChaPoly_BLAKE2s` remains the fallback only if `NKpsk0` cannot be implemented cleanly with the Go library. The fallback is less direct for v1 because the client already knows the host session public key from the invite, but it may be acceptable if Go library support is materially clearer.

The availability test confirms `github.com/flynn/noise` can construct an XX handshake state with PSK placement 3.

## Gate Result

Gate 1 passes when `go test ./internal/securechannel -count=1` passes and this document reflects the actual selected pattern.
