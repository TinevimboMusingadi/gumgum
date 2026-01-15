// Package ttf provides TrueType font parsing from scratch.
// This parses the binary TTF format and extracts glyph outlines,
// metrics, and character mapping tables.
package ttf

import (
	"encoding/binary"
	"fmt"
	"io"
)

// Font represents a parsed TrueType font.
type Font struct {
	// Tables present in the font
	Tables map[string]*Table

	// Parsed table data
	Head   *HeadTable
	Maxp   *MaxpTable
	Hhea   *HheaTable
	Hmtx   *HmtxTable
	Cmap   *CmapTable
	Loca   *LocaTable
	Glyf   *GlyfTable
	Name   *NameTable
	OS2    *OS2Table
	Post   *PostTable
	Kern   *KernTable

	// Font metrics
	UnitsPerEm   uint16
	Ascender     int16
	Descender    int16
	LineGap      int16
	NumGlyphs    uint16
	IndexToLoc   int16 // 0 = short, 1 = long
}

// Table represents a TrueType table entry.
type Table struct {
	Tag      string
	Checksum uint32
	Offset   uint32
	Length   uint32
	Data     []byte
}

// Parse parses a TrueType font from a byte slice.
func Parse(data []byte) (*Font, error) {
	if len(data) < 12 {
		return nil, fmt.Errorf("font data too short")
	}

	font := &Font{
		Tables: make(map[string]*Table),
	}

	// Read offset table
	scalerType := binary.BigEndian.Uint32(data[0:4])
	numTables := binary.BigEndian.Uint16(data[4:6])

	// Validate scaler type (true type or OpenType)
	if scalerType != 0x00010000 && scalerType != 0x4F54544F && scalerType != 0x74727565 {
		return nil, fmt.Errorf("invalid font format: %X", scalerType)
	}

	// Read table directory
	offset := 12
	for i := uint16(0); i < numTables; i++ {
		if offset+16 > len(data) {
			break
		}

		tag := string(data[offset : offset+4])
		table := &Table{
			Tag:      tag,
			Checksum: binary.BigEndian.Uint32(data[offset+4 : offset+8]),
			Offset:   binary.BigEndian.Uint32(data[offset+8 : offset+12]),
			Length:   binary.BigEndian.Uint32(data[offset+12 : offset+16]),
		}

		// Extract table data
		tableEnd := int(table.Offset + table.Length)
		if tableEnd > len(data) {
			tableEnd = len(data)
		}
		if int(table.Offset) < len(data) {
			table.Data = data[table.Offset:tableEnd]
		}

		font.Tables[tag] = table
		offset += 16
	}

	// Parse required tables
	if err := font.parseHead(); err != nil {
		return nil, fmt.Errorf("failed to parse head: %w", err)
	}

	if err := font.parseMaxp(); err != nil {
		return nil, fmt.Errorf("failed to parse maxp: %w", err)
	}

	if err := font.parseHhea(); err != nil {
		return nil, fmt.Errorf("failed to parse hhea: %w", err)
	}

	if err := font.parseHmtx(); err != nil {
		return nil, fmt.Errorf("failed to parse hmtx: %w", err)
	}

	if err := font.parseCmap(); err != nil {
		return nil, fmt.Errorf("failed to parse cmap: %w", err)
	}

	if err := font.parseLoca(); err != nil {
		return nil, fmt.Errorf("failed to parse loca: %w", err)
	}

	if err := font.parseGlyf(); err != nil {
		return nil, fmt.Errorf("failed to parse glyf: %w", err)
	}

	// Optional tables
	font.parseName()
	font.parseOS2()
	font.parsePost()
	font.parseKern()

	return font, nil
}

// ParseReader parses a TrueType font from a reader.
func ParseReader(r io.Reader) (*Font, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// HeadTable contains global font information.
type HeadTable struct {
	Version            uint32
	FontRevision       uint32
	ChecksumAdjustment uint32
	MagicNumber        uint32
	Flags              uint16
	UnitsPerEm         uint16
	Created            int64
	Modified           int64
	XMin               int16
	YMin               int16
	XMax               int16
	YMax               int16
	MacStyle           uint16
	LowestRecPPEM      uint16
	FontDirectionHint  int16
	IndexToLocFormat   int16
	GlyphDataFormat    int16
}

func (f *Font) parseHead() error {
	table := f.Tables["head"]
	if table == nil || len(table.Data) < 54 {
		return fmt.Errorf("head table not found or too short")
	}

	d := table.Data
	f.Head = &HeadTable{
		Version:            binary.BigEndian.Uint32(d[0:4]),
		FontRevision:       binary.BigEndian.Uint32(d[4:8]),
		ChecksumAdjustment: binary.BigEndian.Uint32(d[8:12]),
		MagicNumber:        binary.BigEndian.Uint32(d[12:16]),
		Flags:              binary.BigEndian.Uint16(d[16:18]),
		UnitsPerEm:         binary.BigEndian.Uint16(d[18:20]),
		XMin:               int16(binary.BigEndian.Uint16(d[36:38])),
		YMin:               int16(binary.BigEndian.Uint16(d[38:40])),
		XMax:               int16(binary.BigEndian.Uint16(d[40:42])),
		YMax:               int16(binary.BigEndian.Uint16(d[42:44])),
		MacStyle:           binary.BigEndian.Uint16(d[44:46]),
		LowestRecPPEM:      binary.BigEndian.Uint16(d[46:48]),
		FontDirectionHint:  int16(binary.BigEndian.Uint16(d[48:50])),
		IndexToLocFormat:   int16(binary.BigEndian.Uint16(d[50:52])),
		GlyphDataFormat:    int16(binary.BigEndian.Uint16(d[52:54])),
	}

	f.UnitsPerEm = f.Head.UnitsPerEm
	f.IndexToLoc = f.Head.IndexToLocFormat

	return nil
}

// MaxpTable contains maximum profile data.
type MaxpTable struct {
	Version   uint32
	NumGlyphs uint16
}

func (f *Font) parseMaxp() error {
	table := f.Tables["maxp"]
	if table == nil || len(table.Data) < 6 {
		return fmt.Errorf("maxp table not found or too short")
	}

	d := table.Data
	f.Maxp = &MaxpTable{
		Version:   binary.BigEndian.Uint32(d[0:4]),
		NumGlyphs: binary.BigEndian.Uint16(d[4:6]),
	}

	f.NumGlyphs = f.Maxp.NumGlyphs
	return nil
}

// HheaTable contains horizontal header data.
type HheaTable struct {
	Version             uint32
	Ascender            int16
	Descender           int16
	LineGap             int16
	AdvanceWidthMax     uint16
	MinLeftSideBearing  int16
	MinRightSideBearing int16
	XMaxExtent          int16
	CaretSlopeRise      int16
	CaretSlopeRun       int16
	CaretOffset         int16
	MetricDataFormat    int16
	NumHMetrics         uint16
}

func (f *Font) parseHhea() error {
	table := f.Tables["hhea"]
	if table == nil || len(table.Data) < 36 {
		return fmt.Errorf("hhea table not found or too short")
	}

	d := table.Data
	f.Hhea = &HheaTable{
		Version:             binary.BigEndian.Uint32(d[0:4]),
		Ascender:            int16(binary.BigEndian.Uint16(d[4:6])),
		Descender:           int16(binary.BigEndian.Uint16(d[6:8])),
		LineGap:             int16(binary.BigEndian.Uint16(d[8:10])),
		AdvanceWidthMax:     binary.BigEndian.Uint16(d[10:12]),
		MinLeftSideBearing:  int16(binary.BigEndian.Uint16(d[12:14])),
		MinRightSideBearing: int16(binary.BigEndian.Uint16(d[14:16])),
		XMaxExtent:          int16(binary.BigEndian.Uint16(d[16:18])),
		CaretSlopeRise:      int16(binary.BigEndian.Uint16(d[18:20])),
		CaretSlopeRun:       int16(binary.BigEndian.Uint16(d[20:22])),
		CaretOffset:         int16(binary.BigEndian.Uint16(d[22:24])),
		MetricDataFormat:    int16(binary.BigEndian.Uint16(d[32:34])),
		NumHMetrics:         binary.BigEndian.Uint16(d[34:36]),
	}

	f.Ascender = f.Hhea.Ascender
	f.Descender = f.Hhea.Descender
	f.LineGap = f.Hhea.LineGap

	return nil
}

// BoundingBox returns the font's bounding box.
func (f *Font) BoundingBox() (xMin, yMin, xMax, yMax int16) {
	if f.Head != nil {
		return f.Head.XMin, f.Head.YMin, f.Head.XMax, f.Head.YMax
	}
	return 0, 0, 0, 0
}

// Scale returns a scale factor for the given point size.
func (f *Font) Scale(pointSize float64) float64 {
	return pointSize / float64(f.UnitsPerEm)
}
