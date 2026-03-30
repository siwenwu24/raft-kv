package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"kvraft/fsm"
	"kvraft/httpd"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
)

// config holds all runtime configuration for a single node.
type config struct {
	nodeID           string
	raftAddr         string
	httpAddr         string
	dataDir          string
	bootstrap        bool
	joinAddr         string
	electionTimeout  time.Duration
	heartbeatTimeout time.Duration
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		log.Fatalf("invalid duration for %s: %v", key, err)
	}
	return d
}

func loadConfig() config {
	bootstrapStr := strings.ToLower(envOrDefault("BOOTSTRAP", "false"))
	bootstrap := bootstrapStr == "true" || bootstrapStr == "1" || bootstrapStr == "yes"

	return config{
		nodeID:           envOrDefault("NODE_ID", "node1"),
		raftAddr:         envOrDefault("RAFT_ADDR", "127.0.0.1:7000"),
		httpAddr:         envOrDefault("HTTP_ADDR", "127.0.0.1:8000"),
		dataDir:          envOrDefault("DATA_DIR", "/tmp/kvraft"),
		bootstrap:        bootstrap,
		joinAddr:         os.Getenv("JOIN_ADDR"),
		electionTimeout:  envDuration("ELECTION_TIMEOUT", 300*time.Millisecond),
		heartbeatTimeout: envDuration("HEARTBEAT_TIMEOUT", 150*time.Millisecond),
	}
}

func main() {
	cfg := loadConfig()

	if err := os.MkdirAll(cfg.dataDir, 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}

	// -----------------------------------------------------------------------
	// FSM
	// -----------------------------------------------------------------------
	store := fsm.New()

	// -----------------------------------------------------------------------
	// Raft configuration
	// -----------------------------------------------------------------------
	raftCfg := raft.DefaultConfig()
	raftCfg.LocalID = raft.ServerID(cfg.nodeID)
	raftCfg.ElectionTimeout = cfg.electionTimeout
	raftCfg.HeartbeatTimeout = cfg.heartbeatTimeout
	raftCfg.LeaderLeaseTimeout = cfg.heartbeatTimeout * 2 / 3
	raftCfg.CommitTimeout = 50 * time.Millisecond

	// -----------------------------------------------------------------------
	// Log store (BoltDB)
	// -----------------------------------------------------------------------
	boltPath := filepath.Join(cfg.dataDir, "raft.db")
	logStore, err := raftboltdb.NewBoltStore(boltPath)
	if err != nil {
		log.Fatalf("boltdb: %v", err)
	}
	stableStore := logStore // BoltStore implements both LogStore and StableStore.

	// -----------------------------------------------------------------------
	// Snapshot store
	// -----------------------------------------------------------------------
	snapStore, err := raft.NewFileSnapshotStore(cfg.dataDir, 3, os.Stderr)
	if err != nil {
		log.Fatalf("snapshot store: %v", err)
	}

	// -----------------------------------------------------------------------
	// TCP transport
	// -----------------------------------------------------------------------
	transport, err := raft.NewTCPTransport(cfg.raftAddr, nil, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Fatalf("tcp transport: %v", err)
	}

	// -----------------------------------------------------------------------
	// Create raft node
	// -----------------------------------------------------------------------
	ra, err := raft.NewRaft(raftCfg, store, logStore, stableStore, snapStore, transport)
	if err != nil {
		log.Fatalf("raft.NewRaft: %v", err)
	}

	// -----------------------------------------------------------------------
	// Bootstrap or join
	// -----------------------------------------------------------------------
	if cfg.bootstrap {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raft.ServerID(cfg.nodeID),
					Address: raft.ServerAddress(cfg.raftAddr),
				},
			},
		}
		if f := ra.BootstrapCluster(configuration); f.Error() != nil {
			// Already bootstrapped — ignore.
			log.Printf("bootstrap (ignored): %v", f.Error())
		}
	} else if cfg.joinAddr != "" {
		// Retry joining until the leader is up.
		go func() {
			for {
				if err := joinCluster(cfg.joinAddr, cfg.nodeID, cfg.raftAddr); err != nil {
					log.Printf("join attempt failed: %v — retrying in 2s", err)
					time.Sleep(2 * time.Second)
					continue
				}
				log.Printf("joined cluster via %s", cfg.joinAddr)
				return
			}
		}()
	}

	// -----------------------------------------------------------------------
	// HTTP server
	// -----------------------------------------------------------------------
	// leaderHTTPAddr resolves the current leader's raft address to an HTTP
	// address by convention: same host, HTTP port = raft port - 1000.
	// Override this mapping for production deployments.
	leaderHTTPAddr := func() (string, error) {
		leaderAddr, _ := ra.LeaderWithID()
		if leaderAddr == "" {
			return "", fmt.Errorf("no leader elected")
		}
		return raftToHTTPAddr(string(leaderAddr)), nil
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	httpd.New(r, ra, store, leaderHTTPAddr)

	log.Printf("node %s | raft %s | http %s | bootstrap=%v",
		cfg.nodeID, cfg.raftAddr, cfg.httpAddr, cfg.bootstrap)

	if err := r.Run(cfg.httpAddr); err != nil {
		log.Fatalf("http server: %v", err)
	}
}

// joinCluster sends a POST /join to the node at joinAddr.
func joinCluster(joinAddr, nodeID, raftAddr string) error {
	body, _ := json.Marshal(map[string]string{
		"node_id":   nodeID,
		"raft_addr": raftAddr,
	})
	url := "http://" + joinAddr + "/join"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("join returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// raftToHTTPAddr converts a raft TCP address (host:port) to the corresponding
// HTTP address by subtracting 1000 from the port.
// Convention: raft on :7000 → HTTP on :8000, etc.
// Adjust this function if your deployment uses a different scheme.
func raftToHTTPAddr(raftAddr string) string {
	parts := strings.LastIndex(raftAddr, ":")
	if parts == -1 {
		return raftAddr
	}
	host := raftAddr[:parts]
	portStr := raftAddr[parts+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return raftAddr
	}
	return fmt.Sprintf("%s:%d", host, port+1000)
}
