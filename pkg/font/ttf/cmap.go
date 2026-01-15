package ttf

import (
	"encoding/binary"
	"fmt"
)

// CmapTable contains character to glyph index mappings.
type CmapTable struct {
	Version    uint16
	NumTables  uint16
	Subtables  []CmapSubtable
	BestFormat CmapFormat
}

// CmapSubtable represents a single cmap encoding subtable.
type CmapSubtable struct {
	PlatformID uint16
	EncodingID uint16
	Offset     uint32
	Format     uint16
}

// CmapFormat is the interface for different cmap formats.
type CmapFormat interface {
	GetGlyphID(r rune) uint16
}

// CmapFormat4 is the most common format for Unicode BMP.
type CmapFormat4 struct {
	SegCount      uint16
	EndCode       []uint16
	StartCode     []uint16
	IDDelta       []int16
	IDRangeOffset []uint16
	GlyphIDArray  []uint16
}

// CmapFormat6 handles trimmed table mapping.
type CmapFormat6 struct {
	FirstCode  uint16
	EntryCount uint16
	GlyphIDs   []uint16
}

// CmapFormat12 handles full Unicode (beyond BMP).
type CmapFormat12 struct {
	Groups []CmapGroup
}

// CmapGroup represents a sequential map group in format 12.
type CmapGroup struct {
	StartCharCode uint32
	EndCharCode   uint32
	StartGlyphID  uint32
}

func (f *Font) parseCmap() error {
	table := f.Tables["cmap"]
	if table == nil || len(table.Data) < 4 {
		return fmt.Errorf("cmap table not found or too short")
	}

	d := table.Data
	f.Cmap = &CmapTable{
		Version:   binary.BigEndian.Uint16(d[0:2]),
		NumTables: binary.BigEndian.Uint16(d[2:4]),
	}

	// Parse subtable entries
	offset := 4
	for i := uint16(0); i < f.Cmap.NumTables && offset+8 <= len(d); i++ {
		st := CmapSubtable{
			PlatformID: binary.BigEndian.Uint16(d[offset : offset+2]),
			EncodingID: binary.BigEndian.Uint16(d[offset+2 : offset+4]),
			Offset:     binary.BigEndian.Uint32(d[offset+4 : offset+8]),
		}

		// Read format from subtable
		if int(st.Offset)+2 <= len(d) {
			st.Format = binary.BigEndian.Uint16(d[st.Offset : st.Offset+2])
		}

		f.Cmap.Subtables = append(f.Cmap.Subtables, st)
		offset += 8
	}

	// Find and parse the best subtable
	// Prefer: Format 12 (full Unicode), Format 4 (Unicode BMP), others
	var bestSubtable *CmapSubtable
	bestPriority := -1

	for i := range f.Cmap.Subtables {
		st := &f.Cmap.Subtables[i]
		priority := 0

		// Unicode platform (0) or Windows platform (3) with Unicode encoding
		if st.PlatformID == 0 {
			priority = 3
		} else if st.PlatformID == 3 && (st.EncodingID == 1 || st.EncodingID == 10) {
			priority = 2
		} else if st.PlatformID == 1 && st.EncodingID == 0 {
			priority = 1
		}

		// Prefer format 12 over format 4
		if priority > 0 && st.Format == 12 {
			priority += 10
		}

		if priority > bestPriority {
			bestPriority = priority
			bestSubtable = st
		}
	}

	if bestSubtable == nil {
		return fmt.Errorf("no suitable cmap subtable found")
	}

	// Parse the selected subtable
	return f.parseCmapSubtable(bestSubtable, d)
}

func (f *Font) parseCmapSubtable(st *CmapSubtable, data []byte) error {
	d := data[st.Offset:]

	switch st.Format {
	case 4:
		return f.parseCmapFormat4(d)
	case 6:
		return f.parseCmapFormat6(d)
	case 12:
		return f.parseCmapFormat12(d)
	default:
		return fmt.Errorf("unsupported cmap format: %d", st.Format)
	}
}

func (f *Font) parseCmapFormat4(d []byte) error {
	if len(d) < 14 {
		return fmt.Errorf("format 4 subtable too short")
	}

	length := binary.BigEndian.Uint16(d[2:4])
	segCountX2 := binary.BigEndian.Uint16(d[6:8])
	segCount := segCountX2 / 2

	if int(length) > len(d) {
		return fmt.Errorf("format 4 length exceeds data")
	}

	cmap4 := &CmapFormat4{
		SegCount:      segCount,
		EndCode:       make([]uint16, segCount),
		StartCode:     make([]uint16, segCount),
		IDDelta:       make([]int16, segCount),
		IDRangeOffset: make([]uint16, segCount),
	}

	offset := 14

	// Read endCode array
	for i := uint16(0); i < segCount && offset+2 <= len(d); i++ {
		cmap4.EndCode[i] = binary.BigEndian.Uint16(d[offset : offset+2])
		offset += 2
	}

	offset += 2 // Skip reservedPad

	// Read startCode array
	for i := uint16(0); i < segCount && offset+2 <= len(d); i++ {
		cmap4.StartCode[i] = binary.BigEndian.Uint16(d[offset : offset+2])
		offset += 2
	}

	// Read idDelta array
	for i := uint16(0); i < segCount && offset+2 <= len(d); i++ {
		cmap4.IDDelta[i] = int16(binary.BigEndian.Uint16(d[offset : offset+2]))
		offset += 2
	}

	// Read idRangeOffset array
	idRangeOffsetStart := offset
	for i := uint16(0); i < segCount && offset+2 <= len(d); i++ {
		cmap4.IDRangeOffset[i] = binary.BigEndian.Uint16(d[offset : offset+2])
		offset += 2
	}

	// Read glyphIdArray
	remaining := (int(length) - offset + int(d[2])<<8 + int(d[3]))
	if remaining > 0 && offset < len(d) {
		glyphArrayLen := (len(d) - offset) / 2
		cmap4.GlyphIDArray = make([]uint16, glyphArrayLen)
		for i := 0; i < glyphArrayLen && offset+2 <= len(d); i++ {
			cmap4.GlyphIDArray[i] = binary.BigEndian.Uint16(d[offset : offset+2])
			offset += 2
		}
	}

	// Store reference to idRangeOffset start for glyph lookup
	_ = idRangeOffsetStart

	f.Cmap.BestFormat = cmap4
	return nil
}

func (f *Font) parseCmapFormat6(d []byte) error {
	if len(d) < 10 {
		return fmt.Errorf("format 6 subtable too short")
	}

	cmap6 := &CmapFormat6{
		FirstCode:  binary.BigEndian.Uint16(d[6:8]),
		EntryCount: binary.BigEndian.Uint16(d[8:10]),
	}

	cmap6.GlyphIDs = make([]uint16, cmap6.EntryCount)
	offset := 10
	for i := uint16(0); i < cmap6.EntryCount && offset+2 <= len(d); i++ {
		cmap6.GlyphIDs[i] = binary.BigEndian.Uint16(d[offset : offset+2])
		offset += 2
	}

	f.Cmap.BestFormat = cmap6
	return nil
}

func (f *Font) parseCmapFormat12(d []byte) error {
	if len(d) < 16 {
		return fmt.Errorf("format 12 subtable too short")
	}

	numGroups := binary.BigEndian.Uint32(d[12:16])
	cmap12 := &CmapFormat12{
		Groups: make([]CmapGroup, numGroups),
	}

	offset := 16
	for i := uint32(0); i < numGroups && offset+12 <= len(d); i++ {
		cmap12.Groups[i] = CmapGroup{
			StartCharCode: binary.BigEndian.Uint32(d[offset : offset+4]),
			EndCharCode:   binary.BigEndian.Uint32(d[offset+4 : offset+8]),
			StartGlyphID:  binary.BigEndian.Uint32(d[offset+8 : offset+12]),
		}
		offset += 12
	}

	f.Cmap.BestFormat = cmap12
	return nil
}

// GetGlyphID returns the glyph ID for a Unicode code point.
func (f *Font) GetGlyphID(r rune) uint16 {
	if f.Cmap == nil || f.Cmap.BestFormat == nil {
		return 0
	}
	return f.Cmap.BestFormat.GetGlyphID(r)
}

// GetGlyphID implements CmapFormat for format 4.
func (c *CmapFormat4) GetGlyphID(r rune) uint16 {
	if r > 0xFFFF {
		return 0 // Format 4 only supports BMP
	}

	code := uint16(r)

	// Binary search for the segment
	lo, hi := 0, int(c.SegCount)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		if code > c.EndCode[mid] {
			lo = mid + 1
		} else if code < c.StartCode[mid] {
			hi = mid - 1
		} else {
			// Found the segment
			if c.IDRangeOffset[mid] == 0 {
				return uint16(int(code) + int(c.IDDelta[mid]))
			}

			// Use glyphIdArray
			idx := int(c.IDRangeOffset[mid])/2 + int(code-c.StartCode[mid]) - (int(c.SegCount) - mid)
			if idx >= 0 && idx < len(c.GlyphIDArray) {
				glyphID := c.GlyphIDArray[idx]
				if glyphID != 0 {
					return uint16(int(glyphID) + int(c.IDDelta[mid]))
				}
			}
			return 0
		}
	}

	return 0 // Not found
}

// GetGlyphID implements CmapFormat for format 6.
func (c *CmapFormat6) GetGlyphID(r rune) uint16 {
	if r > 0xFFFF {
		return 0
	}

	code := uint16(r)
	if code < c.FirstCode || code >= c.FirstCode+c.EntryCount {
		return 0
	}

	return c.GlyphIDs[code-c.FirstCode]
}

// GetGlyphID implements CmapFormat for format 12.
func (c *CmapFormat12) GetGlyphID(r rune) uint16 {
	code := uint32(r)

	// Binary search
	lo, hi := 0, len(c.Groups)-1
	for lo <= hi {
		mid := (lo + hi) / 2
		group := &c.Groups[mid]

		if code > group.EndCharCode {
			lo = mid + 1
		} else if code < group.StartCharCode {
			hi = mid - 1
		} else {
			return uint16(group.StartGlyphID + (code - group.StartCharCode))
		}
	}

	return 0
}
