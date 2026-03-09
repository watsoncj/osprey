AGENT   := osprey-agent
SERVER  := osprey-server
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/watsoncj/osprey/internal/buildinfo.Version=$(VERSION)

.PHONY: all clean agent server win64

all: agent server

# Agent binary (daemon-capable, for deployment to endpoints)
agent:
	go build -trimpath -ldflags="-s -w $(LDFLAGS)" -o $(AGENT) ./cmd/agent/

# Cross-compile all binaries for Windows amd64
win64:
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="-s -w $(LDFLAGS)" -o $(AGENT).exe ./cmd/agent/
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags="$(LDFLAGS)" -o $(SERVER).exe ./cmd/server/

# Collection server
server:
	go build -trimpath -ldflags="$(LDFLAGS)" -o $(SERVER) ./cmd/server/

clean:
	rm -f $(AGENT) $(AGENT).exe $(SERVER)
