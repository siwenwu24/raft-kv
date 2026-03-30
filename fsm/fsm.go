package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

// Op constants for log commands.
const (
	OpPut    = "PUT"
	OpDelete = "DELETE"
)

// Command is the payload encoded in every raft.Log entry.
type Command struct {
	Op    string `json:"op"`
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

// KVStore is a mutex-protected in-memory key-value state machine that
// implements raft.FSM.
type KVStore struct {
	mu            sync.RWMutex
	data          map[string]string
	lastAppliedAt time.Time
}

// New returns an empty KVStore ready to be used as a raft FSM.
func New() *KVStore {
	return &KVStore{data: make(map[string]string)}
}

// Apply is called by the raft library when a log entry is committed.
// It must be deterministic and must not block.
func (k *KVStore) Apply(log *raft.Log) interface{} {
	var cmd Command
	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("fsm.Apply: unmarshal: %w", err)
	}

	k.mu.Lock()
	defer k.mu.Unlock()

	switch cmd.Op {
	case OpPut:
		k.data[cmd.Key] = cmd.Value
	case OpDelete:
		delete(k.data, cmd.Key)
	default:
		return fmt.Errorf("fsm.Apply: unknown op %q", cmd.Op)
	}

	k.lastAppliedAt = time.Now()
	return nil
}

// Snapshot returns an FSMSnapshot whose Persist method serialises the
// entire map to JSON and writes it into the raft snapshot sink.
func (k *KVStore) Snapshot() (raft.FSMSnapshot, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Shallow copy so the snapshot is stable even if Apply runs concurrently.
	clone := make(map[string]string, len(k.data))
	for key, val := range k.data {
		clone[key] = val
	}

	b, err := json.Marshal(clone)
	if err != nil {
		return nil, fmt.Errorf("fsm.Snapshot: marshal: %w", err)
	}
	return &fsmSnapshot{data: b}, nil
}

// Restore replaces the FSM state with the snapshot provided by the raft
// library during log compaction catch-up.
func (k *KVStore) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	b, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("fsm.Restore: read: %w", err)
	}

	var data map[string]string
	if err := json.Unmarshal(b, &data); err != nil {
		return fmt.Errorf("fsm.Restore: unmarshal: %w", err)
	}

	k.mu.Lock()
	defer k.mu.Unlock()
	k.data = data
	k.lastAppliedAt = time.Now()
	return nil
}

// Get returns the value for key and whether it was present.
// Safe for concurrent use from HTTP handlers.
func (k *KVStore) Get(key string) (string, bool) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	v, ok := k.data[key]
	return v, ok
}

// LastAppliedAt returns the wall-clock time of the most recent Apply call.
// Used by the HTTP handler to implement bounded-staleness reads.
func (k *KVStore) LastAppliedAt() time.Time {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return k.lastAppliedAt
}

// ---------------------------------------------------------------------------
// fsmSnapshot
// ---------------------------------------------------------------------------

type fsmSnapshot struct {
	data []byte
}

// Persist writes the snapshot bytes into the raft sink and closes it.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if _, err := sink.Write(s.data); err != nil {
		sink.Cancel()
		return fmt.Errorf("fsmSnapshot.Persist: write: %w", err)
	}
	return sink.Close()
}

// Release is a no-op; the snapshot data is GC'd when the struct is collected.
func (s *fsmSnapshot) Release() {}
