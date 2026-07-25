// Package logstore provides a disk-backed, append-only store for request log summaries.
//
// Layout inside dir:
//
//	seg-000001.jsonl  — completed segment (SegmentSize records, one JSON line each)
//	seg-000001.idx    — 8 bytes per record: little-endian int64 byte offset in .jsonl
//	seg-000002.jsonl  — ...
//	current.jsonl     — active segment being appended to
//	current.idx       — active segment index (rebuilt on startup)
//
// Full RequestLog objects (with bodies, WS/SSE frames) are kept in a bounded
// in-memory LRU — they are NOT persisted to disk.
package logstore

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"mockelot/models"
)

const (
	SegmentSize = 50_000 // records per completed segment file
	FullLogCap  = 10_000 // max full RequestLog objects in the LRU
)

// segment represents one .jsonl file with its associated .idx file.
type segment struct {
	path      string   // .jsonl path
	idxPath   string   // .idx path
	count     int      // number of records
	offsets   []int64  // byte offset of each record in the jsonl file
	jsonlFile *os.File // non-nil only for the active (current) segment
	idxFile   *os.File // non-nil only for the active (current) segment
}

// Store is a thread-safe, disk-backed request log store.
type Store struct {
	dir      string
	mu       sync.RWMutex
	segments []*segment // completed, read-only
	current  *segment   // active, append-only

	lruMu   sync.RWMutex
	lruKeys []string                       // insertion-order keys for eviction
	lruMap  map[string]*models.RequestLog  // id → full log
}

// New opens (or creates) the log store rooted at dir.
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("logstore: mkdir %s: %w", dir, err)
	}
	st := &Store{
		dir:    dir,
		lruMap: make(map[string]*models.RequestLog, FullLogCap),
	}
	return st, st.open()
}

// open scans the directory, loads completed segments, and opens the current segment.
func (st *Store) open() error {
	entries, err := os.ReadDir(st.dir)
	if err != nil {
		return fmt.Errorf("logstore: readdir: %w", err)
	}

	var segFiles []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "seg-") && strings.HasSuffix(e.Name(), ".jsonl") {
			segFiles = append(segFiles, e.Name())
		}
	}
	sort.Strings(segFiles)

	for _, name := range segFiles {
		jsonlPath := filepath.Join(st.dir, name)
		idxPath := strings.TrimSuffix(jsonlPath, ".jsonl") + ".idx"
		seg, err := loadCompletedSegment(jsonlPath, idxPath)
		if err != nil {
			return err
		}
		st.segments = append(st.segments, seg)
	}

	cur, err := openCurrentSegment(st.dir)
	if err != nil {
		return err
	}
	st.current = cur
	return nil
}

// Append adds a summary record to the store.
func (st *Store) Append(summary models.RequestLogSummary) error {
	line, err := json.Marshal(summary)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	st.mu.Lock()
	defer st.mu.Unlock()

	if st.current.count >= SegmentSize {
		if err := st.rotate(); err != nil {
			return err
		}
	}

	seg := st.current

	// Record the current file position as the index entry for this record.
	pos, err := seg.jsonlFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	if err := writeIndexEntry(seg.idxFile, pos); err != nil {
		return err
	}
	seg.offsets = append(seg.offsets, pos)

	if _, err := seg.jsonlFile.Write(line); err != nil {
		return err
	}
	seg.count++
	return nil
}

// GetPage returns up to limit summaries starting at the given 0-based global offset
// (oldest-first: offset 0 is the oldest record).
func (st *Store) GetPage(offset, limit int) ([]models.RequestLogSummary, error) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	total := st.totalLocked()
	if offset >= total || limit <= 0 {
		return nil, nil
	}
	if offset+limit > total {
		limit = total - offset
	}

	results := make([]models.RequestLogSummary, 0, limit)
	pos := offset
	remaining := limit
	cumulative := 0

	all := make([]*segment, 0, len(st.segments)+1)
	all = append(all, st.segments...)
	all = append(all, st.current)

	for _, seg := range all {
		if remaining == 0 {
			break
		}
		segEnd := cumulative + seg.count
		if pos >= segEnd {
			cumulative = segEnd
			continue
		}
		localStart := pos - cumulative
		localEnd := localStart + remaining
		if localEnd > seg.count {
			localEnd = seg.count
		}
		records, err := readSegmentRecords(seg, localStart, localEnd)
		if err != nil {
			return nil, err
		}
		results = append(results, records...)
		pos += len(records)
		remaining -= len(records)
		cumulative = segEnd
	}
	return results, nil
}

// GetCount returns the total number of records across all segments.
func (st *Store) GetCount() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return st.totalLocked()
}

func (st *Store) totalLocked() int {
	n := st.current.count
	for _, s := range st.segments {
		n += s.count
	}
	return n
}

// Clear deletes all log data and resets the store.
func (st *Store) Clear() error {
	st.mu.Lock()
	defer st.mu.Unlock()

	if st.current.jsonlFile != nil {
		st.current.jsonlFile.Close()
	}
	if st.current.idxFile != nil {
		st.current.idxFile.Close()
	}

	entries, _ := os.ReadDir(st.dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".jsonl") || strings.HasSuffix(e.Name(), ".idx") {
			os.Remove(filepath.Join(st.dir, e.Name()))
		}
	}

	st.segments = nil
	cur, err := openCurrentSegment(st.dir)
	if err != nil {
		return err
	}
	st.current = cur

	st.lruMu.Lock()
	st.lruKeys = nil
	st.lruMap = make(map[string]*models.RequestLog, FullLogCap)
	st.lruMu.Unlock()

	return nil
}

// PutFull stores a full RequestLog in the bounded in-memory LRU.
func (st *Store) PutFull(id string, log *models.RequestLog) {
	st.lruMu.Lock()
	defer st.lruMu.Unlock()

	if _, exists := st.lruMap[id]; !exists {
		if len(st.lruKeys) >= FullLogCap {
			oldest := st.lruKeys[0]
			st.lruKeys = st.lruKeys[1:]
			delete(st.lruMap, oldest)
		}
		st.lruKeys = append(st.lruKeys, id)
	}
	st.lruMap[id] = log
}

// GetFull retrieves a full RequestLog from the LRU cache.
func (st *Store) GetFull(id string) (*models.RequestLog, bool) {
	st.lruMu.RLock()
	defer st.lruMu.RUnlock()
	l, ok := st.lruMap[id]
	return l, ok
}

// UpdateFull replaces a full log in the LRU (upserts if not present).
func (st *Store) UpdateFull(id string, log *models.RequestLog) {
	st.lruMu.Lock()
	defer st.lruMu.Unlock()
	if _, ok := st.lruMap[id]; ok {
		st.lruMap[id] = log
		return
	}
	if len(st.lruKeys) >= FullLogCap {
		oldest := st.lruKeys[0]
		st.lruKeys = st.lruKeys[1:]
		delete(st.lruMap, oldest)
	}
	st.lruKeys = append(st.lruKeys, id)
	st.lruMap[id] = log
}

// GetAllFull returns all full logs currently in the LRU cache (for export operations).
// Results are in insertion order (oldest-first within the cache window).
func (st *Store) GetAllFull() []*models.RequestLog {
	st.lruMu.RLock()
	defer st.lruMu.RUnlock()
	result := make([]*models.RequestLog, 0, len(st.lruKeys))
	for _, k := range st.lruKeys {
		if l, ok := st.lruMap[k]; ok {
			result = append(result, l)
		}
	}
	return result
}

// DeleteByEndpoint removes all LRU entries for a given endpoint from the cache
// and returns true if any were found. Disk records are not deleted (append-only).
// This is used by per-endpoint clear to free memory; the summary records remain on disk.
func (st *Store) DeleteFullByEndpoint(endpointID string) {
	st.lruMu.Lock()
	defer st.lruMu.Unlock()
	newKeys := st.lruKeys[:0]
	for _, k := range st.lruKeys {
		if l, ok := st.lruMap[k]; ok && l.EndpointID == endpointID {
			delete(st.lruMap, k)
		} else {
			newKeys = append(newKeys, k)
		}
	}
	st.lruKeys = newKeys
}

// --- internal helpers ---

func openCurrentSegment(dir string) (*segment, error) {
	jsonlPath := filepath.Join(dir, "current.jsonl")
	idxPath := filepath.Join(dir, "current.idx")

	jsonlFile, err := os.OpenFile(jsonlPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, fmt.Errorf("logstore: open current.jsonl: %w", err)
	}
	idxFile, err := os.OpenFile(idxPath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		jsonlFile.Close()
		return nil, fmt.Errorf("logstore: open current.idx: %w", err)
	}

	seg := &segment{
		path:      jsonlPath,
		idxPath:   idxPath,
		jsonlFile: jsonlFile,
		idxFile:   idxFile,
	}
	if err := seg.rebuildIndex(); err != nil {
		jsonlFile.Close()
		idxFile.Close()
		return nil, err
	}
	// Seek to end for subsequent appends.
	if _, err := jsonlFile.Seek(0, io.SeekEnd); err != nil {
		return nil, err
	}
	return seg, nil
}

// rebuildIndex scans the jsonl file line-by-line and rebuilds the in-memory offsets
// plus rewrites the idx file to match. Safe to call after a crash.
func (seg *segment) rebuildIndex() error {
	seg.offsets = nil
	seg.count = 0

	if _, err := seg.jsonlFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	scanner := bufio.NewScanner(seg.jsonlFile)
	scanner.Buffer(make([]byte, 4<<20), 4<<20)
	var offset int64
	for scanner.Scan() {
		seg.offsets = append(seg.offsets, offset)
		offset += int64(len(scanner.Bytes())) + 1 // +1 for '\n'
		seg.count++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("logstore: scan current.jsonl: %w", err)
	}

	// Rewrite the idx file to exactly match.
	if err := seg.idxFile.Truncate(0); err != nil {
		return err
	}
	if _, err := seg.idxFile.Seek(0, io.SeekStart); err != nil {
		return err
	}
	for _, off := range seg.offsets {
		if err := writeIndexEntry(seg.idxFile, off); err != nil {
			return err
		}
	}
	return nil
}

func loadCompletedSegment(jsonlPath, idxPath string) (*segment, error) {
	idxData, err := os.ReadFile(idxPath)
	if err != nil {
		return nil, fmt.Errorf("logstore: read %s: %w", idxPath, err)
	}
	n := len(idxData) / 8
	offsets := make([]int64, n)
	for i := range offsets {
		offsets[i] = int64(binary.LittleEndian.Uint64(idxData[i*8:]))
	}
	return &segment{
		path:    jsonlPath,
		idxPath: idxPath,
		count:   n,
		offsets: offsets,
	}, nil
}

func readSegmentRecords(seg *segment, localStart, localEnd int) ([]models.RequestLogSummary, error) {
	if localStart >= localEnd {
		return nil, nil
	}

	var f *os.File
	if seg.jsonlFile != nil {
		// Active segment: use the existing handle (we hold the store's read lock).
		f = seg.jsonlFile
	} else {
		var err error
		f, err = os.Open(seg.path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
	}

	if _, err := f.Seek(seg.offsets[localStart], io.SeekStart); err != nil {
		return nil, err
	}

	results := make([]models.RequestLogSummary, 0, localEnd-localStart)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4<<20), 4<<20)
	for i := localStart; i < localEnd && scanner.Scan(); i++ {
		var rec models.RequestLogSummary
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			return nil, fmt.Errorf("logstore: unmarshal record %d: %w", i, err)
		}
		results = append(results, rec)
	}
	return results, scanner.Err()
}

func (st *Store) rotate() error {
	seg := st.current
	if seg.jsonlFile != nil {
		seg.jsonlFile.Close()
	}
	if seg.idxFile != nil {
		seg.idxFile.Close()
	}

	n := len(st.segments) + 1
	newBase := fmt.Sprintf("seg-%06d", n)
	newJsonl := filepath.Join(st.dir, newBase+".jsonl")
	newIdx := filepath.Join(st.dir, newBase+".idx")
	if err := os.Rename(seg.path, newJsonl); err != nil {
		return err
	}
	if err := os.Rename(seg.idxPath, newIdx); err != nil {
		return err
	}
	seg.path = newJsonl
	seg.idxPath = newIdx
	seg.jsonlFile = nil
	seg.idxFile = nil
	st.segments = append(st.segments, seg)

	cur, err := openCurrentSegment(st.dir)
	if err != nil {
		return err
	}
	st.current = cur
	return nil
}

func writeIndexEntry(f *os.File, offset int64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(offset))
	_, err := f.Write(buf[:])
	return err
}
