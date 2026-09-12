package wal

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fuhrdan/TelemetryForge/internal/domain"
	"github.com/fuhrdan/TelemetryForge/internal/lineage"
)

var (
	ErrDiskPressure = errors.New("edge WAL capacity reached")
	ErrClosed       = errors.New("edge WAL is closed")
)

const (
	segmentHeader       = "TFWAL001"
	segmentHeaderBytes  = int64(len(segmentHeader))
	frameHeaderBytes    = 8
	defaultSegmentBytes = int64(64 << 20)
	defaultMaxBytes     = int64(4 << 30)
	checkpointFile      = "checkpoint.json"
	checkpointVersion   = 1
	defaultPrivateKey   = "lineage.ed25519.pem"
	defaultPublicKey    = "lineage.ed25519.pub.pem"
)

type Config struct {
	Directory        string
	EdgeID           string
	SegmentSizeBytes int64
	MaxBytes         int64
	SigningKeyPath   string
	PublicKeyPath    string
}

type segmentMeta struct {
	path              string
	first             uint64
	last              uint64
	size              int64
	recordCount       uint64
	startPreviousHash string
	sealPath          string
	sealed            bool
}

type checkpoint struct {
	Format            string            `json:"format"`
	FormatVersion     int               `json:"format_version"`
	CommittedSequence uint64            `json:"committed_edge_sequence"`
	SourceSequences   map[string]uint64 `json:"source_sequences,omitempty"`
}

type Store struct {
	mutex           sync.Mutex
	directory       string
	edgeID          string
	segmentBytes    int64
	maxBytes        int64
	segments        []segmentMeta
	current         *os.File
	currentSize     int64
	totalBytes      int64
	nextSequence    uint64
	sourceSequence  map[string]uint64
	committed       uint64
	failed          error
	pressure        bool
	closed          bool
	signer          *lineage.Signer
	lastLineageHash string
	lastSegmentRoot string
	sealedSegments  int
}

func Open(config Config) (*Store, error) {
	directory := strings.TrimSpace(config.Directory)
	if directory == "" {
		directory = "data/edge-wal"
	}
	edgeID := strings.TrimSpace(config.EdgeID)
	if edgeID == "" {
		return nil, errors.New("edge ID is required")
	}
	segmentBytes := config.SegmentSizeBytes
	if segmentBytes <= segmentHeaderBytes+frameHeaderBytes {
		segmentBytes = defaultSegmentBytes
	}
	maxBytes := config.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	if maxBytes < segmentBytes {
		return nil, errors.New("WAL max bytes must be at least one segment")
	}
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return nil, fmt.Errorf("create WAL directory: %w", err)
	}
	privateKeyPath := strings.TrimSpace(config.SigningKeyPath)
	if privateKeyPath == "" {
		privateKeyPath = filepath.Join(directory, defaultPrivateKey)
	}
	publicKeyPath := strings.TrimSpace(config.PublicKeyPath)
	if publicKeyPath == "" {
		publicKeyPath = filepath.Join(directory, defaultPublicKey)
	}
	signer, err := lineage.LoadOrCreateSigner(privateKeyPath, publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("initialize WAL lineage signer: %w", err)
	}
	store := &Store{directory: directory, edgeID: edgeID, segmentBytes: segmentBytes, maxBytes: maxBytes, nextSequence: 1, sourceSequence: make(map[string]uint64), signer: signer}
	if err := store.loadCheckpoint(); err != nil {
		return nil, err
	}
	if err := store.recover(); err != nil {
		return nil, err
	}
	return store, nil
}

func (store *Store) Append(topic string, event domain.Event) (Record, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return Record{}, ErrClosed
	}
	if store.failed != nil {
		return Record{}, store.failed
	}
	if strings.TrimSpace(topic) == "" {
		return Record{}, errors.New("topic is required")
	}
	if err := event.Validate(); err != nil {
		return Record{}, err
	}
	key := event.TenantID + "\x00" + event.Source
	hash, err := EventHash(event)
	if err != nil {
		return Record{}, err
	}
	record := Record{Format: Format, FormatVersion: Version, EdgeID: store.edgeID, Topic: topic, EdgeSequence: store.nextSequence, SourceSequence: store.sourceSequence[key] + 1, AcceptedAt: time.Now().UTC(), PayloadSHA256: hash, LineageVersion: LineageVersion, PreviousRecordHash: store.lastLineageHash, Event: event}
	record.RecordHash, err = ComputeRecordHash(record)
	if err != nil {
		return Record{}, err
	}
	payload, err := json.Marshal(record)
	if err != nil {
		return Record{}, fmt.Errorf("encode WAL record: %w", err)
	}
	frameBytes := int64(frameHeaderBytes + len(payload))
	if frameBytes+segmentHeaderBytes > store.segmentBytes {
		return Record{}, errors.New("WAL record is larger than configured segment size")
	}
	if store.totalBytes+frameBytes > store.maxBytes {
		store.pressure = true
		return Record{}, ErrDiskPressure
	}
	if store.currentSize+frameBytes > store.segmentBytes && store.currentSize > segmentHeaderBytes {
		if store.totalBytes+segmentHeaderBytes+frameBytes > store.maxBytes {
			store.pressure = true
			return Record{}, ErrDiskPressure
		}
		if err := store.rotate(record.EdgeSequence); err != nil {
			store.fail(err)
			return Record{}, err
		}
	}
	var header [frameHeaderBytes]byte
	binary.BigEndian.PutUint32(header[0:4], uint32(len(payload)))
	binary.BigEndian.PutUint32(header[4:8], crc32.ChecksumIEEE(payload))
	if err := writeFull(store.current, header[:]); err != nil {
		store.fail(fmt.Errorf("write WAL frame header: %w", err))
		return Record{}, store.failed
	}
	if err := writeFull(store.current, payload); err != nil {
		store.fail(fmt.Errorf("write WAL frame payload: %w", err))
		return Record{}, store.failed
	}
	if err := store.current.Sync(); err != nil {
		store.fail(fmt.Errorf("sync WAL record: %w", err))
		return Record{}, store.failed
	}
	store.currentSize += frameBytes
	store.totalBytes += frameBytes
	current := &store.segments[len(store.segments)-1]
	current.size = store.currentSize
	if current.recordCount == 0 {
		current.first = record.EdgeSequence
	}
	current.last = record.EdgeSequence
	current.recordCount++
	store.nextSequence++
	store.sourceSequence[key] = record.SourceSequence
	store.lastLineageHash = record.RecordHash
	return record, nil
}

func (store *Store) Pending(limit int) ([]Record, error) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return nil, ErrClosed
	}
	if store.failed != nil {
		return nil, store.failed
	}
	result := make([]Record, 0)
	for _, segment := range store.segments {
		records, _, _, err := scanSegment(segment.path, false)
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if record.EdgeSequence <= store.committed {
				continue
			}
			result = append(result, record)
			if limit > 0 && len(result) >= limit {
				return result, nil
			}
		}
	}
	return result, nil
}

func (store *Store) Commit(sequence uint64) error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return ErrClosed
	}
	if store.failed != nil {
		return store.failed
	}
	if sequence <= store.committed {
		return nil
	}
	if sequence != store.committed+1 {
		return fmt.Errorf("WAL commit gap: committed=%d requested=%d", store.committed, sequence)
	}
	if sequence >= store.nextSequence {
		return fmt.Errorf("cannot commit unknown WAL sequence %d", sequence)
	}
	if err := store.writeCheckpoint(sequence); err != nil {
		store.fail(err)
		return err
	}
	store.committed = sequence
	if err := store.compact(); err != nil {
		store.fail(err)
		return err
	}
	return nil
}

func (store *Store) Ready() error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return ErrClosed
	}
	if store.failed != nil {
		return store.failed
	}
	if store.pressure || store.totalBytes >= store.maxBytes {
		return ErrDiskPressure
	}
	return nil
}

func (store *Store) Stats() Stats {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	last := store.nextSequence - 1
	pending := uint64(0)
	if last > store.committed {
		pending = last - store.committed
	}
	return Stats{Directory: store.directory, Segments: len(store.segments), Bytes: store.totalBytes, MaxBytes: store.maxBytes, PendingRecords: pending, CommittedSequence: store.committed, LastSequence: last, LineageKeyID: store.signer.KeyID(), SealedSegments: store.sealedSegments, LastRecordHash: store.lastLineageHash, LastSegmentRoot: store.lastSegmentRoot}
}

func (store *Store) Close() error {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	if store.closed {
		return nil
	}
	store.closed = true
	if store.current == nil {
		return nil
	}
	if err := store.current.Sync(); err != nil {
		_ = store.current.Close()
		return err
	}
	if err := store.current.Close(); err != nil {
		return err
	}
	store.current = nil
	if len(store.segments) > 0 {
		current := &store.segments[len(store.segments)-1]
		if current.recordCount > 0 && !current.sealed {
			if err := store.sealSegment(current); err != nil {
				return err
			}
		}
	}
	return nil
}

func (store *Store) recover() error {
	sealEntries, err := store.loadSealChain()
	if err != nil {
		return err
	}
	sealsBySegment := make(map[string]lineage.Seal, len(sealEntries))
	for _, seal := range sealEntries {
		sealsBySegment[seal.Segment] = seal
	}

	paths, err := filepath.Glob(filepath.Join(store.directory, "*.tfwal"))
	if err != nil {
		return fmt.Errorf("list WAL segments: %w", err)
	}
	sort.Strings(paths)

	// If compacted segments precede the oldest retained WAL, their latest seal
	// is the cryptographic anchor for the first retained record.
	var previousSequence uint64
	var previousHash string
	var previousRoot string
	priorSealCount := 0
	if len(paths) > 0 {
		firstSequence, err := segmentSequence(paths[0])
		if err != nil {
			return err
		}
		for _, seal := range sealEntries {
			if seal.LastSequence < firstSequence {
				previousHash = seal.LastRecordHash
				previousRoot = seal.MerkleRoot
				previousSequence = seal.LastSequence
				priorSealCount++
			}
		}
	} else if len(sealEntries) > 0 {
		latest := sealEntries[len(sealEntries)-1]
		previousHash = latest.LastRecordHash
		previousRoot = latest.MerkleRoot
		previousSequence = latest.LastSequence
		priorSealCount = len(sealEntries)
	}
	store.lastLineageHash = previousHash
	store.lastSegmentRoot = previousRoot
	store.sealedSegments = priorSealCount

	for index, path := range paths {
		records, size, _, err := scanSegment(path, index == len(paths)-1)
		if err != nil {
			return err
		}
		meta := segmentMeta{path: path, size: size, recordCount: uint64(len(records)), startPreviousHash: store.lastLineageHash, sealPath: sealPathForSegment(path)}
		for _, record := range records {
			if record.EdgeID != store.edgeID {
				return fmt.Errorf("WAL segment %s belongs to edge %q, expected %q", filepath.Base(path), record.EdgeID, store.edgeID)
			}
			if previousSequence != 0 && record.EdgeSequence != previousSequence+1 {
				return fmt.Errorf("WAL sequence gap/corruption: previous=%d current=%d", previousSequence, record.EdgeSequence)
			}
			previousSequence = record.EdgeSequence
			if meta.first == 0 {
				meta.first = record.EdgeSequence
			}
			meta.last = record.EdgeSequence
			key := record.Event.TenantID + "\x00" + record.Event.Source
			if record.SourceSequence > store.sourceSequence[key] {
				store.sourceSequence[key] = record.SourceSequence
			}
		}
		finalHash, leaves, err := VerifyRecordChain(records, store.lastLineageHash)
		if err != nil {
			return fmt.Errorf("verify WAL lineage in %s: %w", filepath.Base(path), err)
		}
		if len(records) > 0 {
			store.lastLineageHash = finalHash
		}

		if seal, ok := sealsBySegment[filepath.Base(path)]; ok {
			if seal.KeyID != store.signer.KeyID() {
				return fmt.Errorf("lineage signer mismatch for %s: seal=%s configured=%s", filepath.Base(path), seal.KeyID, store.signer.KeyID())
			}
			if seal.PreviousSegmentRoot != store.lastSegmentRoot {
				return fmt.Errorf("lineage segment-root discontinuity at %s", filepath.Base(path))
			}
			if err := verifySealAgainstRecords(seal, records, leaves); err != nil {
				return err
			}
			meta.sealed = true
			store.lastSegmentRoot = seal.MerkleRoot
			store.sealedSegments++
		}

		store.segments = append(store.segments, meta)
		store.totalBytes += size
	}

	if previousSequence > 0 {
		store.nextSequence = previousSequence + 1
	}
	if store.committed >= store.nextSequence {
		store.nextSequence = store.committed + 1
	}
	if store.committed > previousSequence && previousSequence != 0 && len(paths) > 0 {
		return fmt.Errorf("WAL checkpoint %d is ahead of recovered sequence %d", store.committed, previousSequence)
	}

	if len(store.segments) == 0 {
		return store.createSegment(store.nextSequence)
	}

	// Upgrade safety: close/seal every completed predecessor. A legacy v1
	// final segment is sealed as an upgrade-time anchor and v2 starts fresh.
	for index := 0; index < len(store.segments)-1; index++ {
		meta := &store.segments[index]
		if meta.recordCount > 0 && !meta.sealed {
			if err := store.sealSegment(meta); err != nil {
				return err
			}
		}
	}
	last := &store.segments[len(store.segments)-1]
	lastRecords, _, _, err := scanSegment(last.path, false)
	if err != nil {
		return err
	}
	legacyLast := false
	for _, record := range lastRecords {
		if record.FormatVersion == LegacyVersion {
			legacyLast = true
			break
		}
	}
	if last.sealed || (legacyLast && last.recordCount > 0) {
		if !last.sealed {
			if err := store.sealSegment(last); err != nil {
				return err
			}
		}
		return store.createSegment(store.nextSequence)
	}

	file, err := os.OpenFile(last.path, os.O_RDWR|os.O_APPEND, 0o640)
	if err != nil {
		return fmt.Errorf("open active WAL segment: %w", err)
	}
	store.current = file
	store.currentSize = last.size
	return nil
}

func (store *Store) loadSealChain() ([]lineage.Seal, error) {
	paths, err := filepath.Glob(filepath.Join(store.directory, "*.tfseal"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	seals := make([]lineage.Seal, 0, len(paths))
	var previousRoot string
	var previousLast uint64
	for _, path := range paths {
		seal, err := lineage.ReadSeal(path)
		if err != nil {
			return nil, fmt.Errorf("read lineage seal %s: %w", filepath.Base(path), err)
		}
		if err := lineage.VerifySeal(seal, nil); err != nil {
			return nil, fmt.Errorf("verify lineage seal %s: %w", filepath.Base(path), err)
		}
		if seal.EdgeID != store.edgeID {
			return nil, fmt.Errorf("lineage seal %s belongs to edge %q, expected %q", filepath.Base(path), seal.EdgeID, store.edgeID)
		}
		if seal.KeyID != store.signer.KeyID() {
			return nil, fmt.Errorf("lineage signer mismatch for %s: seal=%s configured=%s", filepath.Base(path), seal.KeyID, store.signer.KeyID())
		}
		if len(seals) > 0 {
			if seal.PreviousSegmentRoot != previousRoot {
				return nil, fmt.Errorf("lineage seal chain broken at %s", filepath.Base(path))
			}
			if seal.FirstSequence != previousLast+1 {
				return nil, fmt.Errorf("lineage seal sequence gap at %s", filepath.Base(path))
			}
		} else if seal.PreviousSegmentRoot != "" {
			return nil, fmt.Errorf("first retained lineage seal %s unexpectedly references a previous root", filepath.Base(path))
		}
		previousRoot = seal.MerkleRoot
		previousLast = seal.LastSequence
		seals = append(seals, seal)
	}
	return seals, nil
}

func (store *Store) sealSegment(meta *segmentMeta) error {
	if meta == nil || meta.recordCount == 0 || meta.sealed {
		return nil
	}
	records, _, _, err := scanSegment(meta.path, false)
	if err != nil {
		return err
	}
	finalHash, leaves, err := VerifyRecordChain(records, meta.startPreviousHash)
	if err != nil {
		return err
	}
	if len(leaves) == 0 {
		return nil
	}
	root, err := lineage.MerkleRoot(leaves)
	if err != nil {
		return err
	}
	seal, err := store.signer.SignSeal(lineage.Seal{
		EdgeID: store.edgeID, Segment: filepath.Base(meta.path), FirstSequence: records[0].EdgeSequence,
		LastSequence: records[len(records)-1].EdgeSequence, RecordCount: uint64(len(records)),
		FirstRecordHash: leaves[0], LastRecordHash: finalHash, MerkleRoot: root,
		PreviousSegmentRoot: store.lastSegmentRoot, SealedAt: time.Now().UTC(),
	})
	if err != nil {
		return err
	}
	if err := lineage.WriteSeal(meta.sealPath, seal); err != nil {
		return fmt.Errorf("write lineage seal: %w", err)
	}
	meta.sealed = true
	store.lastSegmentRoot = root
	store.sealedSegments++
	return nil
}

func verifySealAgainstRecords(seal lineage.Seal, records []Record, leaves []string) error {
	if len(records) == 0 || len(leaves) == 0 {
		return errors.New("sealed WAL segment contains no records")
	}
	root, err := lineage.MerkleRoot(leaves)
	if err != nil {
		return err
	}
	if seal.FirstSequence != records[0].EdgeSequence || seal.LastSequence != records[len(records)-1].EdgeSequence || seal.RecordCount != uint64(len(records)) {
		return fmt.Errorf("lineage seal sequence/count mismatch for %s", seal.Segment)
	}
	if seal.FirstRecordHash != leaves[0] || seal.LastRecordHash != leaves[len(leaves)-1] || seal.MerkleRoot != root {
		return fmt.Errorf("lineage seal Merkle/hash mismatch for %s", seal.Segment)
	}
	return nil
}

func sealPathForSegment(path string) string {
	return strings.TrimSuffix(path, filepath.Ext(path)) + ".tfseal"
}

func segmentSequence(path string) (uint64, error) {
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	sequence, err := strconv.ParseUint(name, 10, 64)
	if err != nil || sequence == 0 {
		return 0, fmt.Errorf("invalid WAL segment filename %s", filepath.Base(path))
	}
	return sequence, nil
}

func (store *Store) rotate(next uint64) error {
	if store.current != nil {
		if err := store.current.Sync(); err != nil {
			return fmt.Errorf("sync WAL before rotation: %w", err)
		}
		if err := store.current.Close(); err != nil {
			return fmt.Errorf("close WAL before rotation: %w", err)
		}
		store.current = nil
		if len(store.segments) > 0 {
			current := &store.segments[len(store.segments)-1]
			if current.recordCount > 0 && !current.sealed {
				if err := store.sealSegment(current); err != nil {
					return err
				}
			}
		}
	}
	return store.createSegment(next)
}

func (store *Store) createSegment(firstSequence uint64) error {
	path := filepath.Join(store.directory, fmt.Sprintf("%020d.tfwal", firstSequence))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o640)
	if err != nil {
		return fmt.Errorf("create WAL segment: %w", err)
	}
	if err := writeFull(file, []byte(segmentHeader)); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	store.current = file
	store.currentSize = segmentHeaderBytes
	store.totalBytes += segmentHeaderBytes
	store.segments = append(store.segments, segmentMeta{path: path, size: segmentHeaderBytes, startPreviousHash: store.lastLineageHash, sealPath: sealPathForSegment(path)})
	return nil
}

func (store *Store) compact() error {
	if len(store.segments) <= 1 {
		return nil
	}
	kept := make([]segmentMeta, 0, len(store.segments))
	currentPath := store.segments[len(store.segments)-1].path
	for _, segment := range store.segments {
		if segment.path != currentPath && segment.recordCount > 0 && segment.last <= store.committed {
			if err := os.Remove(segment.path); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			store.totalBytes -= segment.size
			continue
		}
		kept = append(kept, segment)
	}
	store.segments = kept
	if store.totalBytes < store.maxBytes {
		store.pressure = false
	}
	return nil
}

func (store *Store) loadCheckpoint() error {
	path := filepath.Join(store.directory, checkpointFile)
	payload, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var cp checkpoint
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cp); err != nil {
		return fmt.Errorf("decode WAL checkpoint: %w", err)
	}
	if cp.Format != Format || cp.FormatVersion != checkpointVersion {
		return errors.New("unsupported WAL checkpoint format/version")
	}
	store.committed = cp.CommittedSequence
	for key, sequence := range cp.SourceSequences {
		store.sourceSequence[key] = sequence
	}
	return nil
}

func (store *Store) writeCheckpoint(sequence uint64) error {
	sequences := make(map[string]uint64, len(store.sourceSequence))
	for key, value := range store.sourceSequence {
		sequences[key] = value
	}
	cp := checkpoint{Format: Format, FormatVersion: checkpointVersion, CommittedSequence: sequence, SourceSequences: sequences}
	payload, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	temporary := filepath.Join(store.directory, checkpointFile+".tmp")
	target := filepath.Join(store.directory, checkpointFile)
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
	return os.Rename(temporary, target)
}

func (store *Store) fail(err error) {
	if store.failed == nil {
		store.failed = err
	}
}

func scanSegment(path string, recoverTail bool) ([]Record, int64, bool, error) {
	flag := os.O_RDONLY
	if recoverTail {
		flag = os.O_RDWR
	}
	file, err := os.OpenFile(path, flag, 0)
	if err != nil {
		return nil, 0, false, err
	}
	defer file.Close()
	header := make([]byte, len(segmentHeader))
	if _, err := io.ReadFull(file, header); err != nil {
		return nil, 0, false, err
	}
	if string(header) != segmentHeader {
		return nil, 0, false, fmt.Errorf("invalid WAL segment header in %s", filepath.Base(path))
	}
	reader := bufio.NewReader(file)
	offset := segmentHeaderBytes
	lastGood := offset
	records := make([]Record, 0)
	truncated := false
	for {
		frameHeader := make([]byte, frameHeaderBytes)
		read, err := io.ReadFull(reader, frameHeader)
		if errors.Is(err, io.EOF) && read == 0 {
			break
		}
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			if !recoverTail {
				return nil, 0, false, fmt.Errorf("truncated WAL frame header in %s", filepath.Base(path))
			}
			truncated = true
			break
		}
		if err != nil {
			return nil, 0, false, err
		}
		length := binary.BigEndian.Uint32(frameHeader[0:4])
		expectedCRC := binary.BigEndian.Uint32(frameHeader[4:8])
		if length == 0 || int64(length) > 1<<30 {
			return nil, 0, false, fmt.Errorf("invalid WAL frame length %d", length)
		}
		payload := make([]byte, int(length))
		if _, err := io.ReadFull(reader, payload); err != nil {
			if (errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF)) && recoverTail {
				truncated = true
				break
			}
			return nil, 0, false, err
		}
		if crc32.ChecksumIEEE(payload) != expectedCRC {
			return nil, 0, false, fmt.Errorf("WAL frame CRC mismatch in %s at offset %d", filepath.Base(path), offset)
		}
		var record Record
		decoder := json.NewDecoder(strings.NewReader(string(payload)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&record); err != nil {
			return nil, 0, false, err
		}
		if err := record.Validate(); err != nil {
			return nil, 0, false, err
		}
		records = append(records, record)
		offset += int64(frameHeaderBytes) + int64(length)
		lastGood = offset
	}
	if truncated {
		if err := file.Truncate(lastGood); err != nil {
			return nil, 0, false, err
		}
		if err := file.Sync(); err != nil {
			return nil, 0, false, err
		}
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, 0, false, err
	}
	return records, stat.Size(), truncated, nil
}

func writeFull(writer io.Writer, payload []byte) error {
	for len(payload) > 0 {
		written, err := writer.Write(payload)
		if err != nil {
			return err
		}
		if written <= 0 {
			return io.ErrShortWrite
		}
		payload = payload[written:]
	}
	return nil
}
