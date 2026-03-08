package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/watsoncj/osprey/internal/model"
	"github.com/watsoncj/osprey/internal/store"
)

func main() {
	listen := flag.String("listen", ":8080", "Address to listen on")
	dataDir := flag.String("data", "./data", "Directory to store reports")
	certFile := flag.String("cert", "", "Path to TLS certificate file (enables HTTPS)")
	keyFile := flag.String("key", "", "Path to TLS private key file")
	apiKey := flag.String("api-key", "", "API key required for report uploads (optional)")
	install := flag.Bool("install", false, "Install server as a system service")
	uninstall := flag.Bool("uninstall", false, "Uninstall server system service")
	flag.Parse()

	if *install {
		if err := installService(*listen, *dataDir, *apiKey); err != nil {
			log.Fatalf("install service: %v", err)
		}
		fmt.Println("Service installed successfully.")
		return
	}

	if *uninstall {
		if err := uninstallService(); err != nil {
			log.Fatalf("uninstall service: %v", err)
		}
		fmt.Println("Service uninstalled successfully.")
		return
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	s := &store.Store{Dir: *dataDir}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/reports", handlePost(s, *apiKey))
	mux.HandleFunc("GET /api/reports", handleList(s))
	mux.HandleFunc("GET /api/reports/{rest...}", handleGet(s))

	log.Printf("Listening on %s (data: %s)", *listen, *dataDir)
	if *certFile != "" && *keyFile != "" {
		log.Printf("TLS enabled")
		if err := http.ListenAndServeTLS(*listen, *certFile, *keyFile, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
	} else {
		if err := http.ListenAndServe(*listen, mux); err != nil {
			log.Fatalf("server: %v", err)
		}
	}
}

func handlePost(s *store.Store, apiKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if apiKey != "" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+apiKey {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}

		var rr model.RunReport
		if err := json.NewDecoder(r.Body).Decode(&rr); err != nil {
			http.Error(w, "invalid JSON: "+err.Error(), http.StatusBadRequest)
			return
		}

		hostname := r.Header.Get("X-Hostname")
		if hostname == "" {
			hostname = rr.Hostname
		}
		if hostname == "" {
			http.Error(w, "missing hostname (set X-Hostname header or hostname in body)", http.StatusBadRequest)
			return
		}

		rr.Hostname = hostname

		if err := s.Save(hostname, rr); err != nil {
			log.Printf("save error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		log.Printf("Stored report from %s", hostname)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleList(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("hostname")

		metas, err := s.List(hostname)
		if err != nil {
			log.Printf("list error: %v", err)
			http.Error(w, "storage error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metas)
	}
}

func handleGet(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rest := r.PathValue("rest")
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			http.Error(w, "use /api/reports/{hostname}/{id}", http.StatusBadRequest)
			return
		}

		hostname, id := parts[0], parts[1]
		rr, err := s.Load(hostname, id)
		if err != nil {
			http.Error(w, "report not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(rr)
	}
}

const serviceName = "osprey-server"
