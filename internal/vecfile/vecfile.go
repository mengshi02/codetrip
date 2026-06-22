package vecfile

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	mmap "github.com/blevesearch/mmap-go"
)

// VectorFile implements a memory-mapped file for storing quantized vectors.
// File layout:
//
//	┌─────────────────────────────────────────┐
//	│ Header (64B):                           │
//	│   magic[4] | version[4] | dim[4]        │
//	│   nodeCount[4] | chunkCount[4]          │
//	│   quantType[4] | reserved[40]           │
//	├─────────────────────────────────────────┤
//	│ Scale table (dim × 4B):                 │
//	│   scale[0] ... scale[dim-1]             │
//	├─────────────────────────────────────────┤
//	│ Offset table (dim × 4B):                │
//	│   offset[0] ... offset[dim-1]           │
//	├─────────────────────────────────────────┤
//	│ Node vectors (nodeCount × dim × 1B):    │
//	│   node[0] ... node[nodeCount-1]         │
//	├─────────────────────────────────────────┤
//	│ Chunk vectors (chunkCount × dim × 1B):  │
//	│   chunk[0] ... chunk[chunkCount-1]      │
//	└─────────────────────────────────────────┘

const (
	VecFileMagic     = "CTVF" // CodeTrip Vector File
	VecFileVersion   = 1
	VecHeaderSize    = 64
	VecFileQuantInt8 = 1
)

// VectorFileHeader is the 64-byte header of the vector file.
type VectorFileHeader struct {
	Magic      [4]byte
	Version    uint32
	Dim        uint32
	NodeCount  uint32
	ChunkCount uint32
	QuantType  uint32
	Reserved   [40]byte
}

// VectorFileWriter builds a quantized vector file on disk.
type VectorFileWriter struct {
	dim       int
	nodeVecs  [][]byte // int8 quantized node vectors
	chunkVecs [][]byte // int8 quantized chunk vectors
	scale     []float32
	offset    []float32
}

// NewVectorFileWriter creates a new writer for the quantized vector file.
func NewVectorFileWriter(dim int, scale, offset []float32) *VectorFileWriter {
	return &VectorFileWriter{
		dim:    dim,
		scale:  scale,
		offset: offset,
	}
}

// AddNodeVector adds a quantized node vector (int8 bytes).
func (w *VectorFileWriter) AddNodeVector(qvec []byte) {
	w.nodeVecs = append(w.nodeVecs, qvec)
}

// AddChunkVector adds a quantized chunk vector (int8 bytes).
func (w *VectorFileWriter) AddChunkVector(qvec []byte) {
	w.chunkVecs = append(w.chunkVecs, qvec)
}

// Write writes the complete vector file to the given path.
func (w *VectorFileWriter) Write(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create vector file directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create vector file: %w", err)
	}
	defer f.Close()

	le := binary.LittleEndian

	// Write header
	var header VectorFileHeader
	copy(header.Magic[:], VecFileMagic)
	header.Version = VecFileVersion
	header.Dim = uint32(w.dim)
	header.NodeCount = uint32(len(w.nodeVecs))
	header.ChunkCount = uint32(len(w.chunkVecs))
	header.QuantType = VecFileQuantInt8

	if err := binary.Write(f, le, &header); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write scale table
	for _, s := range w.scale {
		if err := binary.Write(f, le, s); err != nil {
			return fmt.Errorf("write scale table: %w", err)
		}
	}

	// Write offset table
	for _, o := range w.offset {
		if err := binary.Write(f, le, o); err != nil {
			return fmt.Errorf("write offset table: %w", err)
		}
	}

	// Write node vectors
	for _, qvec := range w.nodeVecs {
		if _, err := f.Write(qvec); err != nil {
			return fmt.Errorf("write node vector: %w", err)
		}
	}

	// Write chunk vectors
	for _, qvec := range w.chunkVecs {
		if _, err := f.Write(qvec); err != nil {
			return fmt.Errorf("write chunk vector: %w", err)
		}
	}

	return nil
}

// VectorFileReader reads a memory-mapped quantized vector file.
type VectorFileReader struct {
	mu       sync.RWMutex
	data     []byte // mmap'd file data
	size     int    // file size
	header   VectorFileHeader
	scale    []float32
	offset   []float32
	nodeOff  int // byte offset where node vectors start
	chunkOff int // byte offset where chunk vectors start
	dim      int
	path     string
	mmapData mmap.MMap // underlying mmap handle for unmap
}

// OpenVectorFile opens and memory-maps a quantized vector file.
func OpenVectorFile(path string) (*VectorFileReader, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open vector file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat vector file: %w", err)
	}
	size := int(info.Size())

	// Read header
	var header VectorFileHeader
	if err := binary.Read(f, binary.LittleEndian, &header); err != nil {
		return nil, fmt.Errorf("read vector file header: %w", err)
	}

	// Validate magic
	if string(header.Magic[:]) != VecFileMagic {
		return nil, fmt.Errorf("invalid vector file magic: %q", string(header.Magic[:]))
	}
	if header.Version != VecFileVersion {
		return nil, fmt.Errorf("unsupported vector file version: %d", header.Version)
	}

	dim := int(header.Dim)

	// Read scale and offset tables
	scale := make([]float32, dim)
	offset := make([]float32, dim)

	if err := binary.Read(f, binary.LittleEndian, scale); err != nil {
		return nil, fmt.Errorf("read scale table: %w", err)
	}
	if err := binary.Read(f, binary.LittleEndian, offset); err != nil {
		return nil, fmt.Errorf("read offset table: %w", err)
	}

	// Compute offsets
	tableSize := dim * 4 // each float32 = 4 bytes
	nodeOff := VecHeaderSize + tableSize*2
	chunkOff := nodeOff + int(header.NodeCount)*dim

	// mmap the file using mmap-go (cross-platform)
	data, err := mmap.Map(f, mmap.RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("mmap vector file: %w", err)
	}

	return &VectorFileReader{
		data:     []byte(data),
		size:     size,
		header:   header,
		scale:    scale,
		offset:   offset,
		nodeOff:  nodeOff,
		chunkOff: chunkOff,
		dim:      dim,
		path:     path,
		mmapData: data,
	}, nil
}

// Close unmaps the file.
func (r *VectorFileReader) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.mmapData != nil {
		err := r.mmapData.Unmap()
		r.data = nil
		r.mmapData = nil
		return err
	}
	return nil
}

// Dim returns the vector dimensionality.
func (r *VectorFileReader) Dim() int { return r.dim }

// NodeCount returns the number of node vectors.
func (r *VectorFileReader) NodeCount() int { return int(r.header.NodeCount) }

// ChunkCount returns the number of chunk vectors.
func (r *VectorFileReader) ChunkCount() int { return int(r.header.ChunkCount) }

// Scale returns the per-dimension scale for dequantization.
func (r *VectorFileReader) Scale() []float32 { return r.scale }

// Offset returns the per-dimension offset for dequantization.
func (r *VectorFileReader) Offset() []float32 { return r.offset }

// NodeQVec returns the int8 quantized vector for a node at the given index.
// The returned slice references the mmap data; do not modify it.
func (r *VectorFileReader) NodeQVec(idx int) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.data == nil || idx < 0 || idx >= int(r.header.NodeCount) {
		return nil
	}
	start := r.nodeOff + idx*r.dim
	end := start + r.dim
	if end > r.size {
		return nil
	}
	return r.data[start:end]
}

// ChunkQVec returns the int8 quantized vector for a chunk at the given index.
// The returned slice references the mmap data; do not modify it.
func (r *VectorFileReader) ChunkQVec(idx int) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.data == nil || idx < 0 || idx >= int(r.header.ChunkCount) {
		return nil
	}
	start := r.chunkOff + idx*r.dim
	end := start + r.dim
	if end > r.size {
		return nil
	}
	return r.data[start:end]
}

// NodeFloat32 returns the dequantized float32 vector for a node at the given index.
func (r *VectorFileReader) NodeFloat32(idx int) []float32 {
	qvec := r.NodeQVec(idx)
	if qvec == nil {
		return nil
	}
	return dequantizeVec(qvec, r.scale, r.offset)
}

// ChunkFloat32 returns the dequantized float32 vector for a chunk at the given index.
func (r *VectorFileReader) ChunkFloat32(idx int) []float32 {
	qvec := r.ChunkQVec(idx)
	if qvec == nil {
		return nil
	}
	return dequantizeVec(qvec, r.scale, r.offset)
}

// dequantizeVec converts int8 quantized bytes to float32 using scale and offset.
func dequantizeVec(qvec []byte, scale, offset []float32) []float32 {
	dim := len(qvec)
	out := make([]float32, dim)
	for i, v := range qvec {
		out[i] = float32(v)*scale[i] + offset[i]
	}
	return out
}

// VectorFilePath returns the file path for a repo's quantized vector file.
func VectorFilePath(dataDir, repo string) string {
	return filepath.Join(dataDir, "vectors", repo+".bin")
}

// VectorFileExists checks if a vector file exists for the given repo.
func VectorFileExists(dataDir, repo string) bool {
	path := VectorFilePath(dataDir, repo)
	_, err := os.Stat(path)
	return err == nil
}

// MmapSliceToString converts a mmap'd byte slice to a string without copying.
// This is unsafe and the caller must ensure the mmap data is not unmapped
// while the string is in use. Use with caution.
func MmapSliceToString(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	return unsafe.String(&data[0], len(data))
}
