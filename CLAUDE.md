# Scouter Client Qt - Project Rules

## Reference Implementation
This project is a Go/Qt6 reimplementation of the Java Scouter client.
**Always refer to the Java source as the authoritative reference** for all implementation work,
not just protocol-related code. This includes views, UI behavior, data flow, commands, and business logic.

### Java source base path
`/Users/nhn/IdeaProjects/scouter`

### Key Java source paths
- **Client views (UI/UX reference)**: `/Users/nhn/IdeaProjects/scouter/scouter.client/src/main/java/scouter/client/`
  - EQ views: `views/EQView.java`, `views/EQCommonView.java`, `group/view/EQGroupView.java`
  - Active Service: `views/ActiveServiceView.java`, `views/ActiveSpeedView.java`
  - XLog: `views/XLogView.java`, `views/XLogCommonView.java`
  - Counter/Chart: `views/CounterView.java`, `counter/views/`
  - Thread detail: `threads/`
- **Protocol packs**: `/Users/nhn/IdeaProjects/scouter/scouter.common/src/main/java/scouter/lang/pack/`
- **IO (DataInputX/DataOutputX)**: `/Users/nhn/IdeaProjects/scouter/scouter.common/src/main/java/scouter/io/`
- **Network commands (server handlers)**: `/Users/nhn/IdeaProjects/scouter/scouter.server/src/main/java/scouter/server/netio/service/`
- **Counter definitions**: `/Users/nhn/IdeaProjects/scouter/scouter.common/src/main/java/scouter/lang/counters/`

### Rules
- When implementing a new view or feature, **first read the corresponding Java client code** to understand the expected behavior, data flow, and UI layout.
- When field order, data types, or serialization format is unclear, read the corresponding Java class to verify.
- Pack types that use blob wrapping (e.g., `XLogPack`) must match the Java `write()`/`read()` pattern exactly.
- Use `d.available() > 0` style optional field checks (Go: `d.Remaining() > 0`) to match Java's versioned field reading.
- When the server returns unexpected data or zero values, check the Java client's handling pattern before assuming a bug.

## Build & Run
- Always use `make` (Makefile) to build and run this project. Do NOT use `go run .` or `go build` directly.
- The Makefile handles required CGo flags (e.g., `CGO_CXXFLAGS="-std=c++17"`) for Qt6 compilation.
