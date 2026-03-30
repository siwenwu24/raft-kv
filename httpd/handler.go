package httpd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"kvraft/fsm"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
)

const (
	// defaultStaleness is the maximum age of a local read for "default" mode.
	defaultStaleness = 500 * time.Millisecond
)

// Handler holds the dependencies for the HTTP layer.
type Handler struct {
	raft  *raft.Raft
	store *fsm.KVStore
	// leaderHTTPAddr returns the HTTP address of the current leader so that
	// write-forwarding knows where to proxy the request.  The caller must
	// supply this function because only the application knows the mapping
	// from raft.ServerAddress → HTTP address.
	leaderHTTPAddr func() (string, error)
}

// New wires up all routes on the supplied gin.Engine and returns the Handler.
func New(r *gin.Engine, ra *raft.Raft, store *fsm.KVStore, leaderHTTPAddr func() (string, error)) *Handler {
	h := &Handler{
		raft:           ra,
		store:          store,
		leaderHTTPAddr: leaderHTTPAddr,
	}

	r.PUT("/kv/:key", h.handlePut)
	r.GET("/kv/:key", h.handleGet)
	r.DELETE("/kv/:key", h.handleDelete)
	r.POST("/join", h.handleJoin)
	r.GET("/health", h.handleHealth)

	return h
}

// ---------------------------------------------------------------------------
// PUT /kv/:key   body: {"value":"..."}
// ---------------------------------------------------------------------------

func (h *Handler) handlePut(c *gin.Context) {
	key := c.Param("key")

	var body struct {
		Value string `json:"value" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.raft.State() != raft.Leader {
		// ShouldBindJSON already consumed the body; reconstruct it for forwarding.
		bodyBytes, _ := json.Marshal(body)
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		h.forwardWrite(c)
		return
	}

	cmd := fsm.Command{Op: fsm.OpPut, Key: key, Value: body.Value}
	if err := h.applyCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "value": body.Value})
}

// ---------------------------------------------------------------------------
// DELETE /kv/:key
// ---------------------------------------------------------------------------

func (h *Handler) handleDelete(c *gin.Context) {
	key := c.Param("key")

	if h.raft.State() != raft.Leader {
		h.forwardWrite(c)
		return
	}

	cmd := fsm.Command{Op: fsm.OpDelete, Key: key}
	if err := h.applyCommand(cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"key": key, "deleted": true})
}

// ---------------------------------------------------------------------------
// GET /kv/:key?level=strong|default|stale
// ---------------------------------------------------------------------------

func (h *Handler) handleGet(c *gin.Context) {
	key := c.Param("key")
	level := c.DefaultQuery("level", "default")

	switch level {
	case "strong":
		// Linearisable: issue a Barrier() so the FSM is caught up to the
		// latest committed index before reading.
		if h.raft.State() != raft.Leader {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "strong reads must be served by the leader",
			})
			return
		}
		if err := h.raft.Barrier(5 * time.Second).Error(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

	case "default":
		// Bounded-staleness: reject if the FSM hasn't applied anything
		// recently enough (i.e., the node is too far behind).
		if time.Since(h.store.LastAppliedAt()) > defaultStaleness {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error": "node is too stale for default-consistency read",
			})
			return
		}

	case "stale":
		// No staleness check — serve immediately from local state.

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("unknown consistency level %q; use strong|default|stale", level),
		})
		return
	}

	val, ok := h.store.Get(key)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"key": key, "error": "not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"key":             key,
		"value":           val,
		"consistency":     level,
		"last_applied_at": h.store.LastAppliedAt(),
	})
}

// ---------------------------------------------------------------------------
// POST /join   body: {"node_id":"...","raft_addr":"..."}
// ---------------------------------------------------------------------------

func (h *Handler) handleJoin(c *gin.Context) {
	if h.raft.State() != raft.Leader {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "not the leader"})
		return
	}

	var body struct {
		NodeID   string `json:"node_id"   binding:"required"`
		RaftAddr string `json:"raft_addr" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	configFuture := h.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(body.NodeID) || srv.Address == raft.ServerAddress(body.RaftAddr) {
			// Already a member.
			c.JSON(http.StatusOK, gin.H{"message": "already a member"})
			return
		}
	}

	f := h.raft.AddVoter(
		raft.ServerID(body.NodeID),
		raft.ServerAddress(body.RaftAddr),
		0, 0,
	)
	if err := f.Error(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"node_id": body.NodeID, "raft_addr": body.RaftAddr})
}

// ---------------------------------------------------------------------------
// GET /health
// ---------------------------------------------------------------------------

func (h *Handler) handleHealth(c *gin.Context) {
	leaderAddr, leaderID := h.raft.LeaderWithID()
	c.JSON(http.StatusOK, gin.H{
		"node_state":      h.raft.State().String(),
		"leader_id":       leaderID,
		"leader_addr":     leaderAddr,
		"term":            h.raft.Stats()["term"],
		"last_applied_at": h.store.LastAppliedAt(),
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// applyCommand serialises cmd and submits it to the raft log.
func (h *Handler) applyCommand(cmd fsm.Command) error {
	b, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	f := h.raft.Apply(b, 5*time.Second)
	if err := f.Error(); err != nil {
		return err
	}
	if resp := f.Response(); resp != nil {
		if err, ok := resp.(error); ok {
			return err
		}
	}
	return nil
}

// forwardWrite proxies the current request to the leader's HTTP address and
// returns the leader's response verbatim to the original caller.
func (h *Handler) forwardWrite(c *gin.Context) {
	leaderHTTP, err := h.leaderHTTPAddr()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "no leader: " + err.Error()})
		return
	}

	url := "http://" + leaderHTTP + c.Request.URL.RequestURI()

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "read body: " + err.Error()})
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, url, bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header = c.Request.Header.Clone()

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "forward to leader: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
}
