package hnsw

import (
	"encoding/binary"
	"math"
)

// QuantType defines the quantization method for vector storage.
type QuantType int

const (
	// QuantNone stores vectors as raw float32 (no quantization).
	QuantNone QuantType = iota
	// QuantInt8 uses scalar quantization: float32 → int8.
	// Each dimension is quantized to int8 using per-dimension scale and offset.
	QuantInt8
)

// QuantParams holds the per-dimension scale and offset for scalar quantization.
// For int8 quantization: int8_val = round((float32_val - offset[i]) / scale[i])
// Dequantization:        float32_val ≈ int8_val * scale[i] + offset[i]
type QuantParams struct {
	Scale  []float32 // per-dimension scale (dim elements)
	Offset []float32 // per-dimension offset (dim elements)
}

// ByteSize returns the byte size of a single quantized vector.
func (q QuantType) ByteSize(dim int) int {
	switch q {
	case QuantInt8:
		return dim // 1 byte per dimension
	default:
		return dim * 4 // float32 = 4 bytes per dimension
	}
}

// TrainQuantParams computes the scale and offset for scalar quantization
// from a set of training vectors. Returns the QuantParams.
// For each dimension: scale = (max - min) / 255, offset = min.
func TrainQuantParams(vectors [][]float32) QuantParams {
	if len(vectors) == 0 {
		return QuantParams{}
	}
	dim := len(vectors[0])
	mins := make([]float32, dim)
	maxes := make([]float32, dim)
	for i := range mins {
		mins[i] = math.MaxFloat32
		maxes[i] = -math.MaxFloat32
	}
	for _, vec := range vectors {
		for i, v := range vec {
			if v < mins[i] {
				mins[i] = v
			}
			if v > maxes[i] {
				maxes[i] = v
			}
		}
	}
	scale := make([]float32, dim)
	offset := make([]float32, dim)
	for i := range mins {
		offset[i] = mins[i]
		rangeVal := maxes[i] - mins[i]
		if rangeVal == 0 {
			rangeVal = 1.0 // avoid division by zero for constant dimensions
		}
		scale[i] = rangeVal / 255.0
	}
	return QuantParams{Scale: scale, Offset: offset}
}

// Quantize converts a float32 vector to an int8 quantized byte slice.
func Quantize(vec []float32, params QuantParams) []byte {
	dim := len(vec)
	out := make([]byte, dim)
	for i, v := range vec {
		normalized := (v - params.Offset[i]) / params.Scale[i]
		// Clamp to [0, 255]
		ival := int(math.Round(float64(normalized)))
		if ival < 0 {
			ival = 0
		} else if ival > 255 {
			ival = 255
		}
		out[i] = byte(ival)
	}
	return out
}

// Dequantize converts an int8 quantized byte slice back to float32.
func Dequantize(qvec []byte, params QuantParams) []float32 {
	dim := len(qvec)
	out := make([]float32, dim)
	for i, v := range qvec {
		out[i] = float32(v)*params.Scale[i] + params.Offset[i]
	}
	return out
}

// Int8CosineDistance computes the cosine distance between two int8 quantized vectors.
// It dequantizes both vectors and then computes cosine distance.
// This is the primary distance function for quantized HNSW graphs.
func Int8CosineDistance(a, b []byte, params QuantParams) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return float32(math.NaN())
	}
	var dotProduct, normA, normB float32
	for i := range a {
		fa := float32(a[i])*params.Scale[i] + params.Offset[i]
		fb := float32(b[i])*params.Scale[i] + params.Offset[i]
		dotProduct += fa * fb
		normA += fa * fa
		normB += fb * fb
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	similarity := dotProduct / (sqrt32(normA) * sqrt32(normB))
	return 1.0 - similarity
}

// Int8CosineDistanceFast computes an approximate cosine distance using int8 dot product.
// It skips full dequantization for speed, using scaled int8 dot product instead.
// This is faster but slightly less accurate than Int8CosineDistance.
func Int8CosineDistanceFast(a, b []byte, params QuantParams) float32 {
	if len(a) == 0 || len(a) != len(b) {
		return float32(math.NaN())
	}
	// Compute int8 dot product and norms
	var dotProduct, normA, normB int32
	for i := range a {
		va := int32(a[i])
		vb := int32(b[i])
		dotProduct += va * vb
		normA += va * va
		normB += vb * vb
	}
	if normA == 0 || normB == 0 {
		return 1.0
	}
	// Approximate cosine similarity from int8 values
	// The scale factors cancel out in the ratio, so we can compute directly
	similarity := float32(dotProduct) / (sqrt32(float32(normA)) * sqrt32(float32(normB)))
	return 1.0 - similarity
}

// QuantizedDistFunc computes distance between two quantized vectors.
type QuantizedDistFunc func(a, b []byte, params QuantParams) float32

// EncodeQuantParams serializes QuantParams to a byte slice.
// Format: [dim:4][scale0:4]...[scaleN:4][offset0:4]...[offsetN:4]
func EncodeQuantParams(params QuantParams) []byte {
	dim := len(params.Scale)
	buf := make([]byte, 4+dim*4+dim*4)
	le := binary.LittleEndian
	le.PutUint32(buf[0:4], uint32(dim))
	off := 4
	for i, s := range params.Scale {
		le.PutUint32(buf[off+i*4:off+i*4+4], math.Float32bits(s))
	}
	off += dim * 4
	for i, o := range params.Offset {
		le.PutUint32(buf[off+i*4:off+i*4+4], math.Float32bits(o))
	}
	return buf
}

// DecodeQuantParams deserializes QuantParams from a byte slice.
func DecodeQuantParams(data []byte) QuantParams {
	if len(data) < 4 {
		return QuantParams{}
	}
	le := binary.LittleEndian
	dim := int(le.Uint32(data[0:4]))
	if len(data) < 4+dim*4+dim*4 {
		return QuantParams{}
	}
	params := QuantParams{
		Scale:  make([]float32, dim),
		Offset: make([]float32, dim),
	}
	off := 4
	for i := range params.Scale {
		params.Scale[i] = math.Float32frombits(le.Uint32(data[off+i*4 : off+i*4+4]))
	}
	off += dim * 4
	for i := range params.Offset {
		params.Offset[i] = math.Float32frombits(le.Uint32(data[off+i*4 : off+i*4+4]))
	}
	return params
}

// sqrt32 returns the square root of a float32.
func sqrt32(v float32) float32 {
	return float32(math.Sqrt(float64(v)))
}