package relayversion

// Version is the product semver stamped into packages and release tags.
// Single source of truth for humans and release tooling is VERSION.md;
// scripts/ci-cd.sh version keeps this constant in sync.
const Version = "0.1.0"

// ProtocolMajor is the Agent Live wire protocol major version.
const ProtocolMajor = 2

// ProtocolMinor is the Agent Live wire protocol minor version.
const ProtocolMinor = 0
