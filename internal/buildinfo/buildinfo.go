package buildinfo

// Version is set at build time via -ldflags:
//
//	-X github.com/watsoncj/osprey/internal/buildinfo.Version=v1.0.0
var Version = "dev"
