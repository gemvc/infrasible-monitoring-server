package relayversion

// relayVersion is the semver stamped into packages and release tags.
// Single source of truth for humans and release tooling is VERSION.md;
// scripts/ci-cd.sh version keeps this constant in sync.
//
// Exported as Version for cmd/ and other packages.
const relayVersion = "0.2.2"

// Version is the exported product semver (same value as relayVersion).
const Version = relayVersion

// ProtocolMajor is the Agent Live wire protocol major version.
const ProtocolMajor = 2

// ProtocolMinor is the Agent Live wire protocol minor version.
const ProtocolMinor = 0
