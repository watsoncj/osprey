package embedded

import _ "embed"

// AgentEXE contains the Windows agent binary, cross-compiled and
// placed here by the Makefile before building the controller.
//
//go:embed agent.exe
var AgentEXE []byte
