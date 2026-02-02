# Scouter Client Qt - Project Rules

## Reference Implementation
This project is a Go/Qt6 reimplementation of the Java Scouter client.
When implementing or debugging protocol-related code (pack serialization, network communication, commands, etc.),
always refer to the Java source at `/Users/nhn/projects/scouter` as the authoritative reference.

### Key Java source paths
- **Protocol packs**: `/Users/nhn/projects/scouter/scouter.common/src/main/java/scouter/lang/pack/`
- **IO (DataInputX/DataOutputX)**: `/Users/nhn/projects/scouter/scouter.common/src/main/java/scouter/io/`
- **Network/commands**: `/Users/nhn/projects/scouter/scouter.common/src/main/java/scouter/lang/pack/` and `/Users/nhn/projects/scouter/scouter.server/src/main/java/scouter/server/netio/service/`
- **Client views**: `/Users/nhn/projects/scouter/scouter.client/src/main/java/scouter/client/`

### Rules
- When field order, data types, or serialization format is unclear, read the corresponding Java class to verify.
- Pack types that use blob wrapping (e.g., `XLogPack`) must match the Java `write()`/`read()` pattern exactly.
- Use `d.available() > 0` style optional field checks (Go: `d.Remaining() > 0`) to match Java's versioned field reading.

## Build & Run
- Always use `make` (Makefile) to build and run this project. Do NOT use `go run .` or `go build` directly.
- The Makefile handles required CGo flags (e.g., `CGO_CXXFLAGS="-std=c++17"`) for Qt6 compilation.
