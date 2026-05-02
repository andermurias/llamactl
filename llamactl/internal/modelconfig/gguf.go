package modelconfig

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// GGUFMeta holds the architecture fields extracted from a GGUF file header.
type GGUFMeta struct {
	Architecture string
	NumLayers    int
	NumHeads     int
	NumKVHeads   int
	HiddenSize   int
	MaxContext   int
}

// ReadGGUFMeta opens a GGUF file and returns its architecture metadata.
// Only the header KV pairs are read — tensor data is never touched.
func ReadGGUFMeta(path string) (GGUFMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return GGUFMeta{}, err
	}
	defer f.Close()
	return parseGGUFHeader(bufio.NewReader(f))
}

// ── GGUF KV value type constants ──────────────────────────────────────────────

const (
	ggufMagic   = "GGUF"
	ggufUINT8   = 0
	ggufINT8    = 1
	ggufUINT16  = 2
	ggufINT16   = 3
	ggufUINT32  = 4
	ggufINT32   = 5
	ggufFLOAT32 = 6
	ggufBOOL    = 7
	ggufSTRING  = 8
	ggufARRAY   = 9
	ggufUINT64  = 10
	ggufINT64   = 11
	ggufFLOAT64 = 12
)

type ggufReader struct{ r *bufio.Reader }

func (g *ggufReader) readU8() (uint8, error)  { return g.r.ReadByte() }
func (g *ggufReader) readU16() (uint16, error) { var v uint16; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readU32() (uint32, error) { var v uint32; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readU64() (uint64, error) { var v uint64; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readI8() (int8, error)   { var v int8; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readI16() (int16, error)  { var v int16; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readI32() (int32, error)  { var v int32; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readI64() (int64, error)  { var v int64; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readF32() (float32, error) { var v float32; return v, binary.Read(g.r, binary.LittleEndian, &v) }
func (g *ggufReader) readF64() (float64, error) { var v float64; return v, binary.Read(g.r, binary.LittleEndian, &v) }

func (g *ggufReader) readString() (string, error) {
	length, err := g.readU64()
	if err != nil {
		return "", err
	}
	buf := make([]byte, length)
	_, err = io.ReadFull(g.r, buf)
	return string(buf), err
}

func (g *ggufReader) skipValue(typ uint32) error {
	switch typ {
	case ggufUINT8, ggufINT8, ggufBOOL:
		_, err := g.readU8()
		return err
	case ggufUINT16, ggufINT16:
		_, err := g.readU16()
		return err
	case ggufUINT32, ggufINT32, ggufFLOAT32:
		_, err := g.readU32()
		return err
	case ggufUINT64, ggufINT64, ggufFLOAT64:
		_, err := g.readU64()
		return err
	case ggufSTRING:
		_, err := g.readString()
		return err
	case ggufARRAY:
		elemType, err := g.readU32()
		if err != nil {
			return err
		}
		count, err := g.readU64()
		if err != nil {
			return err
		}
		for i := uint64(0); i < count; i++ {
			if err := g.skipValue(elemType); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unknown GGUF value type %d", typ)
	}
}

func parseGGUFHeader(br *bufio.Reader) (GGUFMeta, error) {
	g := &ggufReader{r: br}

	magic := make([]byte, 4)
	if _, err := io.ReadFull(br, magic); err != nil {
		return GGUFMeta{}, fmt.Errorf("read magic: %w", err)
	}
	if string(magic) != ggufMagic {
		return GGUFMeta{}, fmt.Errorf("not a GGUF file (magic %q)", string(magic))
	}

	version, err := g.readU32()
	if err != nil {
		return GGUFMeta{}, fmt.Errorf("read version: %w", err)
	}

	var kvCount uint64
	if version == 1 {
		if _, err := g.readU32(); err != nil { // tensor_count (v1: uint32)
			return GGUFMeta{}, err
		}
		kv32, err := g.readU32()
		if err != nil {
			return GGUFMeta{}, err
		}
		kvCount = uint64(kv32)
	} else {
		if _, err := g.readU64(); err != nil { // tensor_count (v2+: uint64)
			return GGUFMeta{}, err
		}
		kvCount, err = g.readU64()
		if err != nil {
			return GGUFMeta{}, err
		}
	}

	var meta GGUFMeta
	for i := uint64(0); i < kvCount; i++ {
		key, err := g.readString()
		if err != nil {
			return GGUFMeta{}, fmt.Errorf("kv[%d] key: %w", i, err)
		}
		valType, err := g.readU32()
		if err != nil {
			return GGUFMeta{}, fmt.Errorf("kv[%d] type: %w", i, err)
		}

		switch {
		case key == "general.architecture" && valType == ggufSTRING:
			meta.Architecture, err = g.readString()
		case strings.HasSuffix(key, ".block_count") && valType == ggufUINT32:
			v, e := g.readU32(); err = e; meta.NumLayers = int(v)
		case strings.HasSuffix(key, ".context_length") && valType == ggufUINT32:
			v, e := g.readU32(); err = e; meta.MaxContext = int(v)
		case strings.HasSuffix(key, ".embedding_length") && valType == ggufUINT32:
			v, e := g.readU32(); err = e; meta.HiddenSize = int(v)
		case strings.HasSuffix(key, ".attention.head_count") &&
			!strings.Contains(key, "kv") && valType == ggufUINT32:
			v, e := g.readU32(); err = e; meta.NumHeads = int(v)
		case strings.HasSuffix(key, ".attention.head_count_kv") && valType == ggufUINT32:
			v, e := g.readU32(); err = e; meta.NumKVHeads = int(v)
		default:
			err = g.skipValue(valType)
		}
		if err != nil {
			return GGUFMeta{}, fmt.Errorf("kv[%d] %q: %w", i, key, err)
		}
	}
	return meta, nil
}
