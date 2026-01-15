package ttf

import (
	"encoding/binary"
	"fmt"
)

// HmtxTable contains horizontal metrics for glyphs.
type HmtxTable struct {
	HMetrics        []LongHorMetric
	LeftSideBearing []int16
}

// LongHorMetric contains advance width and left side bearing.
type LongHorMetric struct {
	AdvanceWidth    uint16
	LeftSideBearing int16
}

func (f *Font) parseHmtx() error {
	table := f.Tables["hmtx"]
	if table == nil {
		return fmt.Errorf("hmtx table not found")
	}

	if f.Hhea == nil || f.Maxp == nil {
		return fmt.Errorf("hhea and maxp must be parsed before hmtx")
	}

	numHMetrics := int(f.Hhea.NumHMetrics)
	numGlyphs := int(f.Maxp.NumGlyphs)
	d := table.Data

	// Validate data length
	minLen := numHMetrics * 4
	if len(d) < minLen {
		return fmt.Errorf("hmtx table too short")
	}

	f.Hmtx = &HmtxTable{
		HMetrics:        make([]LongHorMetric, numHMetrics),
		LeftSideBearing: make([]int16, numGlyphs-numHMetrics),
	}

	// Read long horizontal metrics
	offset := 0
	for i := 0; i < numHMetrics; i++ {
		f.Hmtx.HMetrics[i] = LongHorMetric{
			AdvanceWidth:    binary.BigEndian.Uint16(d[offset : offset+2]),
			LeftSideBearing: int16(binary.BigEndian.Uint16(d[offset+2 : offset+4])),
		}
		offset += 4
	}

	// Read remaining left side bearings (for monospace fonts or when numHMetrics < numGlyphs)
	for i := 0; i < numGlyphs-numHMetrics && offset+2 <= len(d); i++ {
		f.Hmtx.LeftSideBearing[i] = int16(binary.BigEndian.Uint16(d[offset : offset+2]))
		offset += 2
	}

	return nil
}

// GetAdvanceWidth returns the advance width for a glyph.
func (f *Font) GetAdvanceWidth(glyphID uint16) uint16 {
	if f.Hmtx == nil {
		return 0
	}

	if int(glyphID) < len(f.Hmtx.HMetrics) {
		return f.Hmtx.HMetrics[glyphID].AdvanceWidth
	}

	// Glyphs beyond numHMetrics use the last advance width
	if len(f.Hmtx.HMetrics) > 0 {
		return f.Hmtx.HMetrics[len(f.Hmtx.HMetrics)-1].AdvanceWidth
	}

	return 0
}

// GetLeftSideBearing returns the left side bearing for a glyph.
func (f *Font) GetLeftSideBearing(glyphID uint16) int16 {
	if f.Hmtx == nil {
		return 0
	}

	if int(glyphID) < len(f.Hmtx.HMetrics) {
		return f.Hmtx.HMetrics[glyphID].LeftSideBearing
	}

	idx := int(glyphID) - len(f.Hmtx.HMetrics)
	if idx >= 0 && idx < len(f.Hmtx.LeftSideBearing) {
		return f.Hmtx.LeftSideBearing[idx]
	}

	return 0
}

// GetGlyphMetrics returns advance width and left side bearing for a glyph.
func (f *Font) GetGlyphMetrics(glyphID uint16) (advanceWidth uint16, lsb int16) {
	return f.GetAdvanceWidth(glyphID), f.GetLeftSideBearing(glyphID)
}

// GetStringWidth calculates the width of a string in font units.
func (f *Font) GetStringWidth(s string) int {
	var width int
	for _, r := range s {
		glyphID := f.GetGlyphID(rune(r))
		width += int(f.GetAdvanceWidth(glyphID))
	}
	return width
}
