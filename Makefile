OSPREY := osprey
AGENT  := osprey-agent
SERVER := osprey-server

.PHONY: all clean osprey agent server

all: osprey agent server

# CLI tool for local scanning
osprey:
	go build -trimpath -o $(OSPREY) ./cmd/osprey/

# Agent binary (daemon-capable, for deployment to endpoints)
agent:
	go build -trimpath -ldflags="-s -w" -o $(AGENT) ./cmd/agent/

# Cross-compile agent for Windows
agent-windows:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o $(AGENT).exe ./cmd/agent/

# Collection server
server:
	go build -trimpath -o $(SERVER) ./cmd/server/

clean:
	rm -f $(OSPREY) $(AGENT) $(AGENT).exe $(SERVER)
