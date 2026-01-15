package ttf

import (
	"encoding/binary"
	"fmt"
)

// LocaTable contains glyph data offsets.
type LocaTable struct {
	Offsets []uint32
}

// GlyfTable contains glyph outline data.
type GlyfTable struct {
	Data []byte
}

// Glyph represents a parsed glyph.
type Glyph struct {
	// Bounding box
	XMin, YMin, XMax, YMax int16

	// Number of contours (-1 for compound glyphs)
	NumContours int16

	// Simple glyph data
	EndPtsOfContours []uint16
	Instructions     []byte
	Flags            []byte
	XCoordinates     []int16
	YCoordinates     []int16

	// Compound glyph components
	Components []GlyphComponent
}

// GlyphComponent represents a component of a compound glyph.
type GlyphComponent struct {
	GlyphIndex uint16
	Flags      uint16

	// Transformation
	Arg1, Arg2 int16 // dx/dy or point numbers
	Scale      float64
	ScaleX     float64
	ScaleY     float64
	Scale01    float64
	Scale10    float64
}

func (f *Font) parseLoca() error {
	table := f.Tables["loca"]
	if table == nil {
		return fmt.Errorf("loca table not found")
	}

	numGlyphs := int(f.NumGlyphs)
	d := table.Data

	f.Loca = &LocaTable{
		Offsets: make([]uint32, numGlyphs+1),
	}

	if f.IndexToLoc == 0 {
		// Short format: offsets are uint16, multiplied by 2
		for i := 0; i <= numGlyphs && i*2+2 <= len(d); i++ {
			f.Loca.Offsets[i] = uint32(binary.BigEndian.Uint16(d[i*2:i*2+2])) * 2
		}
	} else {
		// Long format: offsets are uint32
		for i := 0; i <= numGlyphs && i*4+4 <= len(d); i++ {
			f.Loca.Offsets[i] = binary.BigEndian.Uint32(d[i*4 : i*4+4])
		}
	}

	return nil
}

func (f *Font) parseGlyf() error {
	table := f.Tables["glyf"]
	if table == nil {
		return fmt.Errorf("glyf table not found")
	}

	f.Glyf = &GlyfTable{
		Data: table.Data,
	}

	return nil
}

// GetGlyph returns the glyph data for a glyph ID.
func (f *Font) GetGlyph(glyphID uint16) (*Glyph, error) {
	if f.Loca == nil || f.Glyf == nil {
		return nil, fmt.Errorf("loca or glyf table not parsed")
	}

	if int(glyphID) >= len(f.Loca.Offsets)-1 {
		return nil, fmt.Errorf("glyph ID %d out of range", glyphID)
	}

	offset := f.Loca.Offsets[glyphID]
	nextOffset := f.Loca.Offsets[glyphID+1]

	// Empty glyph (like space)
	if offset == nextOffset {
		return &Glyph{}, nil
	}

	if int(offset) >= len(f.Glyf.Data) {
		return nil, fmt.Errorf("glyph offset out of range")
	}

	d := f.Glyf.Data[offset:]
	if len(d) < 10 {
		return &Glyph{}, nil
	}

	glyph := &Glyph{
		NumContours: int16(binary.BigEndian.Uint16(d[0:2])),
		XMin:        int16(binary.BigEndian.Uint16(d[2:4])),
		YMin:        int16(binary.BigEndian.Uint16(d[4:6])),
		XMax:        int16(binary.BigEndian.Uint16(d[6:8])),
		YMax:        int16(binary.BigEndian.Uint16(d[8:10])),
	}

	if glyph.NumContours >= 0 {
		return f.parseSimpleGlyph(glyph, d[10:])
	}

	return f.parseCompoundGlyph(glyph, d[10:])
}

// Glyph flags
const (
	flagOnCurve        = 0x01
	flagXShortVector   = 0x02
	flagYShortVector   = 0x04
	flagRepeat         = 0x08
	flagXIsSame        = 0x10
	flagYIsSame        = 0x20
	flagOverlapSimple  = 0x40
)

func (f *Font) parseSimpleGlyph(glyph *Glyph, d []byte) (*Glyph, error) {
	if glyph.NumContours == 0 {
		return glyph, nil
	}

	numContours := int(glyph.NumContours)
	offset := 0

	// Read end points of contours
	glyph.EndPtsOfContours = make([]uint16, numContours)
	for i := 0; i < numContours && offset+2 <= len(d); i++ {
		glyph.EndPtsOfContours[i] = binary.BigEndian.Uint16(d[offset : offset+2])
		offset += 2
	}

	if len(glyph.EndPtsOfContours) == 0 {
		return glyph, nil
	}

	numPoints := int(glyph.EndPtsOfContours[numContours-1]) + 1

	// Read instructions
	if offset+2 > len(d) {
		return glyph, nil
	}
	instructionLen := int(binary.BigEndian.Uint16(d[offset : offset+2]))
	offset += 2

	if offset+instructionLen > len(d) {
		instructionLen = len(d) - offset
	}
	glyph.Instructions = d[offset : offset+instructionLen]
	offset += instructionLen

	// Read flags
	glyph.Flags = make([]byte, numPoints)
	flagIdx := 0
	for flagIdx < numPoints && offset < len(d) {
		flag := d[offset]
		offset++
		glyph.Flags[flagIdx] = flag
		flagIdx++

		// Handle repeat
		if flag&flagRepeat != 0 && offset < len(d) {
			repeatCount := int(d[offset])
			offset++
			for r := 0; r < repeatCount && flagIdx < numPoints; r++ {
				glyph.Flags[flagIdx] = flag
				flagIdx++
			}
		}
	}

	// Read X coordinates
	glyph.XCoordinates = make([]int16, numPoints)
	var x int16
	for i := 0; i < numPoints && offset < len(d); i++ {
		flag := glyph.Flags[i]

		if flag&flagXShortVector != 0 {
			// 1-byte delta
			dx := int16(d[offset])
			offset++
			if flag&flagXIsSame == 0 {
				dx = -dx
			}
			x += dx
		} else if flag&flagXIsSame == 0 {
			// 2-byte delta
			if offset+2 <= len(d) {
				x += int16(binary.BigEndian.Uint16(d[offset : offset+2]))
				offset += 2
			}
		}
		// else: x is unchanged (flagXIsSame set, not short)

		glyph.XCoordinates[i] = x
	}

	// Read Y coordinates
	glyph.YCoordinates = make([]int16, numPoints)
	var y int16
	for i := 0; i < numPoints && offset < len(d); i++ {
		flag := glyph.Flags[i]

		if flag&flagYShortVector != 0 {
			dy := int16(d[offset])
			offset++
			if flag&flagYIsSame == 0 {
				dy = -dy
			}
			y += dy
		} else if flag&flagYIsSame == 0 {
			if offset+2 <= len(d) {
				y += int16(binary.BigEndian.Uint16(d[offset : offset+2]))
				offset += 2
			}
		}

		glyph.YCoordinates[i] = y
	}

	return glyph, nil
}

// Compound glyph flags
const (
	compArg1And2AreWords    = 0x0001
	compArgsAreXYValues     = 0x0002
	compRoundXYToGrid       = 0x0004
	compWeHaveAScale        = 0x0008
	compMoreComponents      = 0x0020
	compWeHaveAnXAndYScale  = 0x0040
	compWeHaveATwoByTwo     = 0x0080
	compWeHaveInstructions  = 0x0100
	compUseMyMetrics        = 0x0200
	compOverlapCompound     = 0x0400
)

func (f *Font) parseCompoundGlyph(glyph *Glyph, d []byte) (*Glyph, error) {
	offset := 0

	for {
		if offset+4 > len(d) {
			break
		}

		flags := binary.BigEndian.Uint16(d[offset : offset+2])
		glyphIndex := binary.BigEndian.Uint16(d[offset+2 : offset+4])
		offset += 4

		comp := GlyphComponent{
			GlyphIndex: glyphIndex,
			Flags:      flags,
			Scale:      1.0,
			ScaleX:     1.0,
			ScaleY:     1.0,
		}

		// Read arguments
		if flags&compArg1And2AreWords != 0 {
			if offset+4 <= len(d) {
				comp.Arg1 = int16(binary.BigEndian.Uint16(d[offset : offset+2]))
				comp.Arg2 = int16(binary.BigEndian.Uint16(d[offset+2 : offset+4]))
				offset += 4
			}
		} else {
			if offset+2 <= len(d) {
				comp.Arg1 = int16(int8(d[offset]))
				comp.Arg2 = int16(int8(d[offset+1]))
				offset += 2
			}
		}

		// Read transformation
		if flags&compWeHaveAScale != 0 {
			if offset+2 <= len(d) {
				comp.Scale = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset : offset+2]))
				comp.ScaleX = comp.Scale
				comp.ScaleY = comp.Scale
				offset += 2
			}
		} else if flags&compWeHaveAnXAndYScale != 0 {
			if offset+4 <= len(d) {
				comp.ScaleX = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset : offset+2]))
				comp.ScaleY = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset+2 : offset+4]))
				offset += 4
			}
		} else if flags&compWeHaveATwoByTwo != 0 {
			if offset+8 <= len(d) {
				comp.ScaleX = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset : offset+2]))
				comp.Scale01 = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset+2 : offset+4]))
				comp.Scale10 = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset+4 : offset+6]))
				comp.ScaleY = fixed2dot14ToFloat(binary.BigEndian.Uint16(d[offset+6 : offset+8]))
				offset += 8
			}
		}

		glyph.Components = append(glyph.Components, comp)

		if flags&compMoreComponents == 0 {
			break
		}
	}

	return glyph, nil
}

// fixed2dot14ToFloat converts a 2.14 fixed-point number to float64.
func fixed2dot14ToFloat(v uint16) float64 {
	return float64(int16(v)) / 16384.0
}

// IsOnCurve returns true if the point at index i is on the curve.
func (g *Glyph) IsOnCurve(i int) bool {
	if i < 0 || i >= len(g.Flags) {
		return false
	}
	return g.Flags[i]&flagOnCurve != 0
}

// IsCompound returns true if this is a compound glyph.
func (g *Glyph) IsCompound() bool {
	return g.NumContours < 0
}

// GetContour returns the points for a specific contour.
func (g *Glyph) GetContour(contourIdx int) (xs, ys []int16, onCurve []bool) {
	if contourIdx < 0 || contourIdx >= len(g.EndPtsOfContours) {
		return nil, nil, nil
	}

	start := 0
	if contourIdx > 0 {
		start = int(g.EndPtsOfContours[contourIdx-1]) + 1
	}
	end := int(g.EndPtsOfContours[contourIdx]) + 1

	numPoints := end - start
	xs = make([]int16, numPoints)
	ys = make([]int16, numPoints)
	onCurve = make([]bool, numPoints)

	for i := 0; i < numPoints; i++ {
		idx := start + i
		if idx < len(g.XCoordinates) {
			xs[i] = g.XCoordinates[idx]
		}
		if idx < len(g.YCoordinates) {
			ys[i] = g.YCoordinates[idx]
		}
		if idx < len(g.Flags) {
			onCurve[i] = g.Flags[idx]&flagOnCurve != 0
		}
	}

	return xs, ys, onCurve
}
