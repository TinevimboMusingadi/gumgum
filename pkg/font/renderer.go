// Package font provides font loading and glyph rendering.
package font

import (
	"gumgum/pkg/font/ttf"
	"gumgum/pkg/graphics"
)

// Renderer converts font glyphs to graphics paths.
type Renderer struct {
	font   *ttf.Font
	scale  float64
	hScale float64 // Horizontal scaling (text state)
}

// NewRenderer creates a new font renderer.
func NewRenderer(font *ttf.Font) *Renderer {
	return &Renderer{
		font:   font,
		scale:  1.0,
		hScale: 1.0,
	}
}

// SetScale sets the scale factor (point size / units per em).
func (r *Renderer) SetScale(pointSize float64) {
	r.scale = pointSize / float64(r.font.UnitsPerEm)
}

// SetHorizontalScale sets the horizontal scaling percentage.
func (r *Renderer) SetHorizontalScale(percentage float64) {
	r.hScale = percentage / 100.0
}

// GlyphToPath converts a glyph to a graphics path.
func (r *Renderer) GlyphToPath(glyphID uint16) (*graphics.Path, error) {
	glyph, err := r.font.GetGlyph(glyphID)
	if err != nil {
		return nil, err
	}

	if glyph.IsCompound() {
		return r.compoundGlyphToPath(glyph)
	}

	return r.simpleGlyphToPath(glyph), nil
}

// simpleGlyphToPath converts a simple glyph to a path.
func (r *Renderer) simpleGlyphToPath(glyph *ttf.Glyph) *graphics.Path {
	path := graphics.NewPath()

	if glyph.NumContours <= 0 {
		return path
	}

	scale := r.scale
	hScale := r.hScale

	// Process each contour
	for c := 0; c < int(glyph.NumContours); c++ {
		xs, ys, onCurve := glyph.GetContour(c)
		if len(xs) == 0 {
			continue
		}

		numPoints := len(xs)

		// Find first on-curve point or insert one
		firstOnCurve := -1
		for i := 0; i < numPoints; i++ {
			if onCurve[i] {
				firstOnCurve = i
				break
			}
		}

		var startX, startY float64
		var startIdx int

		if firstOnCurve >= 0 {
			startIdx = firstOnCurve
			startX = float64(xs[startIdx]) * scale * hScale
			startY = float64(ys[startIdx]) * scale
		} else {
			// All off-curve: start at midpoint between first and last
			startX = float64(xs[0]+xs[numPoints-1]) / 2 * scale * hScale
			startY = float64(ys[0]+ys[numPoints-1]) / 2 * scale
			startIdx = 0
		}

		path.MoveTo(startX, startY)

		// Walk through points
		i := (startIdx + 1) % numPoints
		for count := 0; count < numPoints; count++ {
			x := float64(xs[i]) * scale * hScale
			y := float64(ys[i]) * scale

			if onCurve[i] {
				path.LineTo(x, y)
			} else {
				// Off-curve point - need to handle quadratic Bezier
				// Look at next point
				nextI := (i + 1) % numPoints
				nextX := float64(xs[nextI]) * scale * hScale
				nextY := float64(ys[nextI]) * scale

				var endX, endY float64
				if onCurve[nextI] {
					endX, endY = nextX, nextY
					count++ // Skip next point
					i = nextI
				} else {
					// Two consecutive off-curve: midpoint is on curve
					endX = (x + nextX) / 2
					endY = (y + nextY) / 2
				}

				// Convert quadratic to cubic Bezier
				// Current point is the start
				cur := path.CurrentPoint()
				cp1x := cur.X + 2.0/3.0*(x-cur.X)
				cp1y := cur.Y + 2.0/3.0*(y-cur.Y)
				cp2x := endX + 2.0/3.0*(x-endX)
				cp2y := endY + 2.0/3.0*(y-endY)

				path.CurveTo(cp1x, cp1y, cp2x, cp2y, endX, endY)
			}

			i = (i + 1) % numPoints
		}

		path.Close()
	}

	return path
}

// compoundGlyphToPath converts a compound glyph to a path.
func (r *Renderer) compoundGlyphToPath(glyph *ttf.Glyph) (*graphics.Path, error) {
	result := graphics.NewPath()

	for _, comp := range glyph.Components {
		// Get component glyph
		compPath, err := r.GlyphToPath(comp.GlyphIndex)
		if err != nil {
			continue
		}

		// Apply transformation
		scale := r.scale
		hScale := r.hScale

		dx := float64(comp.Arg1) * scale * hScale
		dy := float64(comp.Arg2) * scale

		// Create transformation matrix
		m := graphics.Identity()

		// Apply component transformation (scale/rotation)
		if comp.Scale != 1.0 || comp.ScaleX != 1.0 || comp.ScaleY != 1.0 ||
			comp.Scale01 != 0 || comp.Scale10 != 0 {

			tm := graphics.Matrix{
				comp.ScaleX, comp.Scale01,
				comp.Scale10, comp.ScaleY,
				0, 0,
			}
			m = m.Multiply(tm)
		}

		// Apply translation
		m = m.Multiply(graphics.Translate(dx, dy))

		// Transform component path
		transformed := compPath.Transform(m)

		// Add to result
		for _, seg := range transformed.Segments {
			switch seg.Op {
			case graphics.PathOpMoveTo:
				if len(seg.Points) > 0 {
					result.MoveTo(seg.Points[0].X, seg.Points[0].Y)
				}
			case graphics.PathOpLineTo:
				if len(seg.Points) > 0 {
					result.LineTo(seg.Points[0].X, seg.Points[0].Y)
				}
			case graphics.PathOpCurveTo:
				if len(seg.Points) >= 3 {
					result.CurveTo(
						seg.Points[0].X, seg.Points[0].Y,
						seg.Points[1].X, seg.Points[1].Y,
						seg.Points[2].X, seg.Points[2].Y,
					)
				}
			case graphics.PathOpClose:
				result.Close()
			}
		}
	}

	return result, nil
}

// RenderString renders a string to a path at the given position.
func (r *Renderer) RenderString(s string, x, y float64) *graphics.Path {
	result := graphics.NewPath()
	currentX := x

	for _, runeValue := range s {
		glyphID := r.font.GetGlyphID(runeValue)

		// Get glyph path
		glyphPath, err := r.GlyphToPath(glyphID)
		if err == nil && !glyphPath.IsEmpty() {
			// Translate glyph to current position
			translated := glyphPath.Transform(graphics.Translate(currentX, y))

			// Append to result
			for _, seg := range translated.Segments {
				switch seg.Op {
				case graphics.PathOpMoveTo:
					if len(seg.Points) > 0 {
						result.MoveTo(seg.Points[0].X, seg.Points[0].Y)
					}
				case graphics.PathOpLineTo:
					if len(seg.Points) > 0 {
						result.LineTo(seg.Points[0].X, seg.Points[0].Y)
					}
				case graphics.PathOpCurveTo:
					if len(seg.Points) >= 3 {
						result.CurveTo(
							seg.Points[0].X, seg.Points[0].Y,
							seg.Points[1].X, seg.Points[1].Y,
							seg.Points[2].X, seg.Points[2].Y,
						)
					}
				case graphics.PathOpClose:
					result.Close()
				}
			}
		}

		// Advance position
		advanceWidth := float64(r.font.GetAdvanceWidth(glyphID)) * r.scale * r.hScale
		currentX += advanceWidth
	}

	return result
}

// GetStringWidth returns the width of a string in scaled units.
func (r *Renderer) GetStringWidth(s string) float64 {
	var width float64
	var prevGlyphID uint16

	for i, runeValue := range s {
		glyphID := r.font.GetGlyphID(runeValue)

		// Add kerning
		if i > 0 {
			kern := float64(r.font.GetKerning(prevGlyphID, glyphID)) * r.scale * r.hScale
			width += kern
		}

		// Add advance width
		advance := float64(r.font.GetAdvanceWidth(glyphID)) * r.scale * r.hScale
		width += advance

		prevGlyphID = glyphID
	}

	return width
}

// Metrics returns font metrics in scaled units.
type Metrics struct {
	Ascender   float64
	Descender  float64
	LineHeight float64
	XHeight    float64
	CapHeight  float64
}

// GetMetrics returns the font metrics at the current scale.
func (r *Renderer) GetMetrics() Metrics {
	font := r.font
	scale := r.scale

	m := Metrics{
		Ascender:   float64(font.Ascender) * scale,
		Descender:  float64(font.Descender) * scale,
		LineHeight: float64(font.Ascender-font.Descender+font.LineGap) * scale,
	}

	if font.OS2 != nil {
		m.XHeight = float64(font.OS2.SxHeight) * scale
		m.CapHeight = float64(font.OS2.SCapHeight) * scale
	}

	return m
}
