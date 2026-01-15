package ttf

import (
	"encoding/binary"
	"unicode/utf16"
)

// NameTable contains font naming information.
type NameTable struct {
	Format       uint16
	Count        uint16
	StringOffset uint16
	Records      []NameRecord
}

// NameRecord contains a single name entry.
type NameRecord struct {
	PlatformID uint16
	EncodingID uint16
	LanguageID uint16
	NameID     uint16
	Length     uint16
	Offset     uint16
	Value      string
}

// Name IDs
const (
	NameCopyright         = 0
	NameFontFamily        = 1
	NameFontSubfamily     = 2
	NameUniqueID          = 3
	NameFullName          = 4
	NameVersion           = 5
	NamePostScriptName    = 6
	NameTrademark         = 7
	NameManufacturer      = 8
	NameDesigner          = 9
	NameDescription       = 10
	NameVendorURL         = 11
	NameDesignerURL       = 12
	NameLicense           = 13
	NameLicenseURL        = 14
	NamePreferredFamily   = 16
	NamePreferredSubfamily = 17
)

func (f *Font) parseName() error {
	table := f.Tables["name"]
	if table == nil || len(table.Data) < 6 {
		return nil // Optional table
	}

	d := table.Data
	f.Name = &NameTable{
		Format:       binary.BigEndian.Uint16(d[0:2]),
		Count:        binary.BigEndian.Uint16(d[2:4]),
		StringOffset: binary.BigEndian.Uint16(d[4:6]),
	}

	offset := 6
	stringData := d[f.Name.StringOffset:]

	for i := uint16(0); i < f.Name.Count && offset+12 <= len(d); i++ {
		rec := NameRecord{
			PlatformID: binary.BigEndian.Uint16(d[offset : offset+2]),
			EncodingID: binary.BigEndian.Uint16(d[offset+2 : offset+4]),
			LanguageID: binary.BigEndian.Uint16(d[offset+4 : offset+6]),
			NameID:     binary.BigEndian.Uint16(d[offset+6 : offset+8]),
			Length:     binary.BigEndian.Uint16(d[offset+8 : offset+10]),
			Offset:     binary.BigEndian.Uint16(d[offset+10 : offset+12]),
		}
		offset += 12

		// Extract string value
		if int(rec.Offset)+int(rec.Length) <= len(stringData) {
			strBytes := stringData[rec.Offset : rec.Offset+rec.Length]
			rec.Value = decodeString(strBytes, rec.PlatformID, rec.EncodingID)
		}

		f.Name.Records = append(f.Name.Records, rec)
	}

	return nil
}

func decodeString(data []byte, platformID, encodingID uint16) string {
	// Platform 3 (Windows) with encoding 1 (Unicode BMP) uses UTF-16BE
	if platformID == 3 && encodingID == 1 {
		if len(data)%2 != 0 {
			return ""
		}
		u16 := make([]uint16, len(data)/2)
		for i := range u16 {
			u16[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
		}
		return string(utf16.Decode(u16))
	}

	// Platform 0 (Unicode) also uses UTF-16BE typically
	if platformID == 0 {
		if len(data)%2 != 0 {
			return string(data)
		}
		u16 := make([]uint16, len(data)/2)
		for i := range u16 {
			u16[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
		}
		return string(utf16.Decode(u16))
	}

	// Platform 1 (Macintosh) - assume MacRoman for now
	return string(data)
}

// GetName returns a name value by ID.
func (f *Font) GetName(nameID uint16) string {
	if f.Name == nil {
		return ""
	}

	// Prefer Windows/Unicode platform
	for _, rec := range f.Name.Records {
		if rec.NameID == nameID && rec.PlatformID == 3 && rec.EncodingID == 1 {
			return rec.Value
		}
	}

	// Fall back to any platform
	for _, rec := range f.Name.Records {
		if rec.NameID == nameID && rec.Value != "" {
			return rec.Value
		}
	}

	return ""
}

// FamilyName returns the font family name.
func (f *Font) FamilyName() string {
	name := f.GetName(NamePreferredFamily)
	if name == "" {
		name = f.GetName(NameFontFamily)
	}
	return name
}

// FullName returns the full font name.
func (f *Font) FullName() string {
	return f.GetName(NameFullName)
}

// PostScriptName returns the PostScript name.
func (f *Font) PostScriptName() string {
	return f.GetName(NamePostScriptName)
}

// OS2Table contains OS/2 and Windows metrics.
type OS2Table struct {
	Version            uint16
	XAvgCharWidth      int16
	UsWeightClass      uint16
	UsWidthClass       uint16
	FsType             uint16
	YSubscriptXSize    int16
	YSubscriptYSize    int16
	YSubscriptXOffset  int16
	YSubscriptYOffset  int16
	YSuperscriptXSize  int16
	YSuperscriptYSize  int16
	YSuperscriptXOffset int16
	YSuperscriptYOffset int16
	YStrikeoutSize     int16
	YStrikeoutPosition int16
	SFamilyClass       int16
	STypoAscender      int16
	STypoDescender     int16
	STypoLineGap       int16
	UsWinAscent        uint16
	UsWinDescent       uint16
	SxHeight           int16
	SCapHeight         int16
}

func (f *Font) parseOS2() error {
	table := f.Tables["OS/2"]
	if table == nil || len(table.Data) < 68 {
		return nil // Optional table
	}

	d := table.Data
	f.OS2 = &OS2Table{
		Version:            binary.BigEndian.Uint16(d[0:2]),
		XAvgCharWidth:      int16(binary.BigEndian.Uint16(d[2:4])),
		UsWeightClass:      binary.BigEndian.Uint16(d[4:6]),
		UsWidthClass:       binary.BigEndian.Uint16(d[6:8]),
		FsType:             binary.BigEndian.Uint16(d[8:10]),
		YSubscriptXSize:    int16(binary.BigEndian.Uint16(d[10:12])),
		YSubscriptYSize:    int16(binary.BigEndian.Uint16(d[12:14])),
		YSubscriptXOffset:  int16(binary.BigEndian.Uint16(d[14:16])),
		YSubscriptYOffset:  int16(binary.BigEndian.Uint16(d[16:18])),
		YSuperscriptXSize:  int16(binary.BigEndian.Uint16(d[18:20])),
		YSuperscriptYSize:  int16(binary.BigEndian.Uint16(d[20:22])),
		YSuperscriptXOffset: int16(binary.BigEndian.Uint16(d[22:24])),
		YSuperscriptYOffset: int16(binary.BigEndian.Uint16(d[24:26])),
		YStrikeoutSize:     int16(binary.BigEndian.Uint16(d[26:28])),
		YStrikeoutPosition: int16(binary.BigEndian.Uint16(d[28:30])),
		SFamilyClass:       int16(binary.BigEndian.Uint16(d[30:32])),
	}

	// Version 0+ fields
	if len(d) >= 78 {
		f.OS2.STypoAscender = int16(binary.BigEndian.Uint16(d[68:70]))
		f.OS2.STypoDescender = int16(binary.BigEndian.Uint16(d[70:72]))
		f.OS2.STypoLineGap = int16(binary.BigEndian.Uint16(d[72:74]))
		f.OS2.UsWinAscent = binary.BigEndian.Uint16(d[74:76])
		f.OS2.UsWinDescent = binary.BigEndian.Uint16(d[76:78])
	}

	// Version 2+ fields
	if f.OS2.Version >= 2 && len(d) >= 96 {
		f.OS2.SxHeight = int16(binary.BigEndian.Uint16(d[86:88]))
		f.OS2.SCapHeight = int16(binary.BigEndian.Uint16(d[88:90]))
	}

	return nil
}

// PostTable contains PostScript information.
type PostTable struct {
	Version            uint32
	ItalicAngle        float64
	UnderlinePosition  int16
	UnderlineThickness int16
	IsFixedPitch       uint32
}

func (f *Font) parsePost() error {
	table := f.Tables["post"]
	if table == nil || len(table.Data) < 32 {
		return nil
	}

	d := table.Data
	f.Post = &PostTable{
		Version:            binary.BigEndian.Uint32(d[0:4]),
		UnderlinePosition:  int16(binary.BigEndian.Uint16(d[8:10])),
		UnderlineThickness: int16(binary.BigEndian.Uint16(d[10:12])),
		IsFixedPitch:       binary.BigEndian.Uint32(d[12:16]),
	}

	// Italic angle is a 16.16 fixed-point number
	intPart := int16(binary.BigEndian.Uint16(d[4:6]))
	fracPart := binary.BigEndian.Uint16(d[6:8])
	f.Post.ItalicAngle = float64(intPart) + float64(fracPart)/65536.0

	return nil
}

// KernTable contains kerning pairs.
type KernTable struct {
	Version  uint16
	NumPairs uint16
	Pairs    map[uint32]int16 // Key: (left << 16) | right
}

func (f *Font) parseKern() error {
	table := f.Tables["kern"]
	if table == nil || len(table.Data) < 4 {
		return nil
	}

	d := table.Data
	f.Kern = &KernTable{
		Version: binary.BigEndian.Uint16(d[0:2]),
		Pairs:   make(map[uint32]int16),
	}

	numTables := binary.BigEndian.Uint16(d[2:4])
	offset := 4

	for t := uint16(0); t < numTables && offset+6 <= len(d); t++ {
		// Read subtable header
		// version := binary.BigEndian.Uint16(d[offset : offset+2])
		length := binary.BigEndian.Uint16(d[offset+2 : offset+4])
		coverage := binary.BigEndian.Uint16(d[offset+4 : offset+6])
		offset += 6

		// Only handle format 0 horizontal kerning
		format := coverage >> 8
		if format != 0 {
			offset += int(length) - 6
			continue
		}

		if offset+8 > len(d) {
			break
		}

		numPairs := binary.BigEndian.Uint16(d[offset : offset+2])
		offset += 8 // Skip searchRange, entrySelector, rangeShift

		for p := uint16(0); p < numPairs && offset+6 <= len(d); p++ {
			left := binary.BigEndian.Uint16(d[offset : offset+2])
			right := binary.BigEndian.Uint16(d[offset+2 : offset+4])
			value := int16(binary.BigEndian.Uint16(d[offset+4 : offset+6]))
			offset += 6

			key := uint32(left)<<16 | uint32(right)
			f.Kern.Pairs[key] = value
		}
	}

	return nil
}

// GetKerning returns the kerning adjustment between two glyphs.
func (f *Font) GetKerning(left, right uint16) int16 {
	if f.Kern == nil {
		return 0
	}

	key := uint32(left)<<16 | uint32(right)
	return f.Kern.Pairs[key]
}

// IsFixedPitch returns true if the font is monospace.
func (f *Font) IsFixedPitch() bool {
	if f.Post != nil {
		return f.Post.IsFixedPitch != 0
	}
	return false
}

// Weight returns the font weight (100-900).
func (f *Font) Weight() int {
	if f.OS2 != nil {
		return int(f.OS2.UsWeightClass)
	}
	return 400 // Normal weight default
}
