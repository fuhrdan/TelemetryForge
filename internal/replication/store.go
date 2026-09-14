package replication

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/fuhrdan/TelemetryForge/internal/wal"
)

const replicaHeader = "TFREP001"
const replicaFrameHeaderBytes = 8
const defaultReplicaMaxBytes = int64(8 << 30)
const compactReleaseStride = uint64(256)

var (
	ErrReplicaConflict = errors.New("conflicting replica sequence")
	ErrReplicaPressure = errors.New("replica store capacity reached")
)

type StoreConfig struct {
	Directory string
	MaxBytes  int64
}

type StoreStats struct {
	Directory       string `json:"directory"`
	Bytes           int64  `json:"bytes"`
	MaxBytes        int64  `json:"max_bytes"`
	Origins         int    `json:"origins"`
	RetainedRecords int    `json:"retained_records"`
}

type releaseCheckpoint struct {
	Format          string `json:"format"`
	FormatVersion   int    `json:"format_version"`
	EdgeID          string `json:"origin_edge_id"`
	ReleasedThrough uint64 `json:"released_through"`
}

// Store is the crash-safe receiver-side replica log. Records retain the origin
// edge sequence; a repeated identical record is acknowledged idempotently.
type Store struct {
	mutex        sync.Mutex
	directory    string
	maxBytes     int64
	totalBytes   int64
	fingerprints map[string]map[uint64]string
	released     map[string]uint64
	compacted    map[string]uint64
	closed       bool
}

func OpenStore(directory string) (*Store, error) {
	return OpenStoreWithConfig(StoreConfig{Directory: directory})
}

func OpenStoreWithConfig(config StoreConfig) (*Store, error) {
	directory := strings.TrimSpace(config.Directory)
	if directory == "" {
		directory = "data/edge-replicas"
	}
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultReplicaMaxBytes
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create replica directory: %w", err)
	}
	store := &Store{
		directory:    directory,
		maxBytes:     maxBytes,
		fingerprints: make(map[string]map[uint64]string),
		released:     make(map[string]uint64),
		compacted:    make(map[string]uint64),
	}
	if err := store.loadReleaseCheckpoints(); err != nil {
		return nil, err
	}
	paths, err := filepath.Glob(filepath.Join(directory, "*.tfreplica"))
	if err != nil {
		return nil, fmt.Errorf("list replica logs: %w", err)
	}
	for _, path := range paths {
		if err := store.recoverFile(path); err != nil {
			return nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		store.totalBytes += info.Size()
	}
	return store, nil
}

func (store *Store) Put(record wal.Record) (bool, error) {
	if err := record.Validate(); err != nil {
		return false, err
	}
	fingerprint, payload, err := fingerprintRecord(record)
	if err != nil {
		return false, err
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return false, errors.New("replica store is closed")
	}
	if record.EdgeSequence <= store.released[record.EdgeID] {
		return true, nil
	}
	bySequence := store.fingerprints[record.EdgeID]
	if bySequence == nil {
		bySequence = make(map[uint64]string)
		store.fingerprints[record.EdgeID] = bySequence
	}
	if existing, ok := bySequence[record.EdgeSequence]; ok {
		if existing != fingerprint {
			return false, ErrReplicaConflict
		}
		return true, nil
	}

	path := store.path(record.EdgeID)
	newlyCreated := false
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		newlyCreated = true
	} else if err != nil {
		return false, err
	}
	additional := int64(replicaFrameHeaderBytes + len(payload))
	if newlyCreated {
		additional += int64(len(replicaHeader))
	}
	if store.totalBytes+additional > store.maxBytes {
		if err := store.compactReleased(); err != nil {
			return false, err
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			newlyCreated = true
		} else if err != nil {
			return false, err
		} else {
			newlyCreated = false
		}
		additional = int64(replicaFrameHeaderBytes + len(payload))
		if newlyCreated {
			additional += int64(len(replicaHeader))
		}
		if store.totalBytes+additional > store.maxBytes {
			return false, ErrReplicaPressure
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return false, fmt.Errorf("open replica log: %w", err)
	}
	defer file.Close()
	if newlyCreated {
		if _, err := file.Write([]byte(replicaHeader)); err != nil {
			return false, fmt.Errorf("write replica header: %w", err)
		}
	}
	var header [replicaFrameHeaderBytes]byte
	binary.BigEndian.PutUint32(header[0:4], uint32(len(payload)))
	binary.BigEndian.PutUint32(header[4:8], crc32.ChecksumIEEE(payload))
	if err := writeFull(file, header[:]); err != nil {
		return false, fmt.Errorf("write replica frame header: %w", err)
	}
	if err := writeFull(file, payload); err != nil {
		return false, fmt.Errorf("write replica frame payload: %w", err)
	}
	if err := file.Sync(); err != nil {
		return false, fmt.Errorf("sync replica log: %w", err)
	}
	if newlyCreated {
		if err := syncDirectory(store.directory); err != nil {
			return false, err
		}
	}
	store.totalBytes += additional
	bySequence[record.EdgeSequence] = fingerprint
	return false, nil
}

// Release records that an origin has durably checkpointed downstream delivery.
// The release checkpoint is durable even when physical compaction is deferred.
func (store *Store) Release(edgeID string, through uint64) error {
	edgeID = strings.TrimSpace(edgeID)
	if edgeID == "" {
		return errors.New("origin edge ID is required")
	}
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return errors.New("replica store is closed")
	}
	if through <= store.released[edgeID] {
		return nil
	}
	if err := store.writeReleaseCheckpoint(edgeID, through); err != nil {
		return err
	}
	store.released[edgeID] = through
	if bySequence := store.fingerprints[edgeID]; bySequence != nil {
		for sequence := range bySequence {
			if sequence <= through {
				delete(bySequence, sequence)
			}
		}
	}
	if through-store.compacted[edgeID] >= compactReleaseStride || store.totalBytes >= (store.maxBytes*3)/4 {
		if err := store.compactEdge(edgeID); err != nil {
			return err
		}
	}
	return nil
}

func (store *Store) Count(edgeID string) int {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	return len(store.fingerprints[edgeID])
}

func (store *Store) Stats() StoreStats {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	retained := 0
	for _, values := range store.fingerprints {
		retained += len(values)
	}
	return StoreStats{Directory: store.directory, Bytes: store.totalBytes, MaxBytes: store.maxBytes, Origins: len(store.fingerprints), RetainedRecords: retained}
}

func (store *Store) Close() error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	store.closed = true
	return nil
}

func (store *Store) loadReleaseCheckpoints() error {
	paths, err := filepath.Glob(filepath.Join(store.directory, "*.release.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var checkpoint releaseCheckpoint
		decoder := json.NewDecoder(strings.NewReader(string(payload)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&checkpoint); err != nil {
			return fmt.Errorf("decode replica release checkpoint: %w", err)
		}
		if checkpoint.Format != "telemetryforge-edge-replica-release" || checkpoint.FormatVersion != 1 || strings.TrimSpace(checkpoint.EdgeID) == "" {
			return errors.New("unsupported replica release checkpoint")
		}
		if checkpoint.ReleasedThrough > store.released[checkpoint.EdgeID] {
			store.released[checkpoint.EdgeID] = checkpoint.ReleasedThrough
		}
	}
	return nil
}

func (store *Store) recoverFile(path string) error {
	records, truncateTo, err := scanReplicaFile(path)
	if err != nil {
		return err
	}
	if truncateTo >= 0 {
		if err := os.Truncate(path, truncateTo); err != nil {
			return err
		}
	}
	for _, record := range records {
		if record.EdgeSequence <= store.released[record.EdgeID] {
			continue
		}
		fingerprint, _, err := fingerprintRecord(record)
		if err != nil {
			return err
		}
		bySequence := store.fingerprints[record.EdgeID]
		if bySequence == nil {
			bySequence = make(map[uint64]string)
			store.fingerprints[record.EdgeID] = bySequence
		}
		if existing, ok := bySequence[record.EdgeSequence]; ok && existing != fingerprint {
			return ErrReplicaConflict
		}
		bySequence[record.EdgeSequence] = fingerprint
	}
	return nil
}

func scanReplicaFile(path string) ([]wal.Record, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, -1, fmt.Errorf("open replica log: %w", err)
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	header := make([]byte, len(replicaHeader))
	if _, err := io.ReadFull(reader, header); err != nil {
		return nil, -1, fmt.Errorf("read replica header: %w", err)
	}
	if string(header) != replicaHeader {
		return nil, -1, errors.New("unsupported replica log header")
	}
	offset := int64(len(replicaHeader))
	records := make([]wal.Record, 0)
	for {
		var frameHeader [replicaFrameHeaderBytes]byte
		if _, err := io.ReadFull(reader, frameHeader[:]); err != nil {
			if errors.Is(err, io.EOF) {
				return records, -1, nil
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return records, offset, nil
			}
			return nil, -1, err
		}
		size := binary.BigEndian.Uint32(frameHeader[0:4])
		expectedCRC := binary.BigEndian.Uint32(frameHeader[4:8])
		if size == 0 || size > 16<<20 {
			return nil, -1, errors.New("invalid replica frame size")
		}
		payload := make([]byte, size)
		if _, err := io.ReadFull(reader, payload); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return records, offset, nil
			}
			return nil, -1, err
		}
		if crc32.ChecksumIEEE(payload) != expectedCRC {
			return nil, -1, errors.New("replica frame CRC mismatch")
		}
		var record wal.Record
		if err := json.Unmarshal(payload, &record); err != nil {
			return nil, -1, fmt.Errorf("decode replica record: %w", err)
		}
		if err := record.Validate(); err != nil {
			return nil, -1, fmt.Errorf("validate replica record: %w", err)
		}
		records = append(records, record)
		offset += int64(replicaFrameHeaderBytes) + int64(size)
	}
}

func (store *Store) writeReleaseCheckpoint(edgeID string, through uint64) error {
	checkpoint := releaseCheckpoint{Format: "telemetryforge-edge-replica-release", FormatVersion: 1, EdgeID: edgeID, ReleasedThrough: through}
	payload, err := json.Marshal(checkpoint)
	if err != nil {
		return err
	}
	path := store.releasePath(edgeID)
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if err := writeFull(file, payload); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	return syncDirectory(store.directory)
}

func (store *Store) compactReleased() error {
	for edgeID, through := range store.released {
		if through > store.compacted[edgeID] {
			if err := store.compactEdge(edgeID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *Store) compactEdge(edgeID string) error {
	path := store.path(edgeID)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		store.compacted[edgeID] = store.released[edgeID]
		return nil
	}
	if err != nil {
		return err
	}
	records, _, err := scanReplicaFile(path)
	if err != nil {
		return err
	}
	retained := make([]wal.Record, 0, len(records))
	for _, record := range records {
		if record.EdgeSequence > store.released[edgeID] {
			retained = append(retained, record)
		}
	}
	if len(retained) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		store.totalBytes -= info.Size()
		if store.totalBytes < 0 {
			store.totalBytes = 0
		}
		store.compacted[edgeID] = store.released[edgeID]
		return syncDirectory(store.directory)
	}
	temporary := path + ".compact"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o640)
	if err != nil {
		return err
	}
	if err := writeFull(file, []byte(replicaHeader)); err != nil {
		_ = file.Close()
		return err
	}
	for _, record := range retained {
		_, payload, err := fingerprintRecord(record)
		if err != nil {
			_ = file.Close()
			return err
		}
		var header [replicaFrameHeaderBytes]byte
		binary.BigEndian.PutUint32(header[0:4], uint32(len(payload)))
		binary.BigEndian.PutUint32(header[4:8], crc32.ChecksumIEEE(payload))
		if err := writeFull(file, header[:]); err != nil {
			_ = file.Close()
			return err
		}
		if err := writeFull(file, payload); err != nil {
			_ = file.Close()
			return err
		}
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	newInfo, err := os.Stat(temporary)
	if err != nil {
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		return err
	}
	if err := syncDirectory(store.directory); err != nil {
		return err
	}
	store.totalBytes = store.totalBytes - info.Size() + newInfo.Size()
	store.compacted[edgeID] = store.released[edgeID]
	return nil
}

func (store *Store) path(edgeID string) string {
	sum := sha256.Sum256([]byte(edgeID))
	return filepath.Join(store.directory, hex.EncodeToString(sum[:12])+".tfreplica")
}

func (store *Store) releasePath(edgeID string) string {
	sum := sha256.Sum256([]byte(edgeID))
	return filepath.Join(store.directory, hex.EncodeToString(sum[:12])+".release.json")
}

func fingerprintRecord(record wal.Record) (string, []byte, error) {
	payload, err := json.Marshal(record)
	if err != nil {
		return "", nil, err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), payload, nil
}

func writeFull(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		written, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		payload = payload[written:]
	}
	return nil
}

func syncDirectory(directory string) error {
	// Go cannot fsync directory handles through os.File on Windows. Replica
	// files themselves are fsynced before rename, so treat the directory flush
	// as a POSIX-only durability reinforcement rather than failing all writes.
	if runtime.GOOS == "windows" {
		return nil
	}
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync replica directory: %w", err)
	}
	return nil
}
