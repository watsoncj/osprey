AGENT_EMBED := internal/embedded/agent.exe
CONTROLLER  := browser-forensics

.PHONY: all clean agent controller

all: controller

# Cross-compile the agent for Windows amd64
agent:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o $(AGENT_EMBED) ./cmd/agent/

# Build the controller (embeds the Windows agent)
controller: agent
	go build -trimpath -o $(CONTROLLER) ./cmd/browser-forensics/

# Local-only build (no embedded agent, for development)
dev:
	go build -o $(CONTROLLER) ./cmd/browser-forensics/

clean:
	rm -f $(CONTROLLER) $(AGENT_EMBED)
	echo "placeholder" > $(AGENT_EMBED)
