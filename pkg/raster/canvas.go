// Package raster provides rasterization of PDF graphics to images.
package raster

import (
	"image"
	"image/color"
	"image/draw"
	"math"

	"gumgum/pkg/graphics"
	pathpkg "gumgum/pkg/path"

	"golang.org/x/image/vector"
)

// Canvas represents a drawing surface for rasterization.
type Canvas struct {
	img    *image.RGBA
	width  int
	height int
	dpi    float64

	// Default background
	background color.Color
}

// NewCanvas creates a new canvas with the given dimensions.
func NewCanvas(width, height int) *Canvas {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with white background
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	return &Canvas{
		img:        img,
		width:      width,
		height:     height,
		dpi:        72,
		background: color.White,
	}
}

// NewCanvasWithDPI creates a canvas sized for the given page dimensions and DPI.
func NewCanvasWithDPI(pageWidth, pageHeight, dpi float64) *Canvas {
	width := int(math.Ceil(pageWidth * dpi / 72))
	height := int(math.Ceil(pageHeight * dpi / 72))

	c := NewCanvas(width, height)
	c.dpi = dpi
	return c
}

// Image returns the underlying RGBA image.
func (c *Canvas) Image() *image.RGBA {
	return c.img
}

// Width returns the canvas width in pixels.
func (c *Canvas) Width() int {
	return c.width
}

// Height returns the canvas height in pixels.
func (c *Canvas) Height() int {
	return c.height
}

// DPI returns the canvas DPI.
func (c *Canvas) DPI() float64 {
	return c.dpi
}

// Clear fills the canvas with the background color.
func (c *Canvas) Clear() {
	draw.Draw(c.img, c.img.Bounds(), &image.Uniform{c.background}, image.Point{}, draw.Src)
}

// SetBackground sets the background color.
func (c *Canvas) SetBackground(col color.Color) {
	c.background = col
}

// Fill fills a path with the given color using the specified fill rule.
func (c *Canvas) Fill(path *graphics.Path, col color.Color, rule graphics.FillRule) {
	if path.IsEmpty() {
		return
	}

	// Create rasterizer
	r := &vector.Rasterizer{}
	r.Reset(c.width, c.height)

	// Convert and add path
	pathpkg.ToVector(path, r)

	// Draw based on fill rule
	var src image.Image = &image.Uniform{col}

	if rule == graphics.FillRuleEvenOdd {
		r.DrawOp = draw.Src
	}

	r.Draw(c.img, c.img.Bounds(), src, image.Point{})
}

// Stroke draws the outline of a path with the given style.
func (c *Canvas) Stroke(path *graphics.Path, col color.Color, width float64, cap graphics.LineCap, join graphics.LineJoin) {
	if path.IsEmpty() {
		return
	}

	// Convert path to stroke path (outline the stroke)
	strokePath := strokeToPath(path, width, cap, join)

	// Fill the stroke path
	c.Fill(strokePath, col, graphics.FillRuleNonZero)
}

// strokeToPath converts a stroke to a fillable path.
func strokeToPath(path *graphics.Path, width float64, cap graphics.LineCap, join graphics.LineJoin) *graphics.Path {
	halfWidth := width / 2
	result := graphics.NewPath()

	// Simple implementation: expand each segment by half width
	var segments []strokeSegment

	// Build segments
	var current, start graphics.Point
	for _, seg := range path.Segments {
		switch seg.Op {
		case graphics.PathOpMoveTo:
			if len(seg.Points) > 0 {
				current = seg.Points[0]
				start = current
			}
		case graphics.PathOpLineTo:
			if len(seg.Points) > 0 {
				end := seg.Points[0]
				segments = append(segments, strokeSegment{
					start: current,
					end:   end,
				})
				current = end
			}
		case graphics.PathOpCurveTo:
			// Approximate curve with line segments
			if len(seg.Points) >= 3 {
				end := seg.Points[2]
				segments = append(segments, strokeSegment{
					start: current,
					end:   end,
				})
				current = end
			}
		case graphics.PathOpClose:
			if current != start {
				segments = append(segments, strokeSegment{
					start: current,
					end:   start,
				})
			}
			current = start
		}
	}

	// Generate outline
	if len(segments) == 0 {
		return result
	}

	// Left side
	for _, seg := range segments {
		dx := seg.end.X - seg.start.X
		dy := seg.end.Y - seg.start.Y
		length := math.Sqrt(dx*dx + dy*dy)
		if length == 0 {
			continue
		}

		// Perpendicular unit vector
		nx := -dy / length
		ny := dx / length

		x1 := seg.start.X + nx*halfWidth
		y1 := seg.start.Y + ny*halfWidth
		x2 := seg.end.X + nx*halfWidth
		y2 := seg.end.Y + ny*halfWidth

		if result.IsEmpty() {
			result.MoveTo(x1, y1)
		} else {
			result.LineTo(x1, y1)
		}
		result.LineTo(x2, y2)
	}

	// Add end cap
	addCap(result, segments[len(segments)-1].end, segments[len(segments)-1], halfWidth, cap, false)

	// Right side (reverse)
	for i := len(segments) - 1; i >= 0; i-- {
		seg := segments[i]
		dx := seg.end.X - seg.start.X
		dy := seg.end.Y - seg.start.Y
		length := math.Sqrt(dx*dx + dy*dy)
		if length == 0 {
			continue
		}

		// Perpendicular (opposite side)
		nx := dy / length
		ny := -dx / length

		x1 := seg.end.X + nx*halfWidth
		y1 := seg.end.Y + ny*halfWidth
		x2 := seg.start.X + nx*halfWidth
		y2 := seg.start.Y + ny*halfWidth

		result.LineTo(x1, y1)
		result.LineTo(x2, y2)
	}

	// Add start cap
	addCap(result, segments[0].start, segments[0], halfWidth, cap, true)

	result.Close()
	return result
}

type strokeSegment struct {
	start, end graphics.Point
}

func addCap(path *graphics.Path, pt graphics.Point, seg strokeSegment, halfWidth float64, cap graphics.LineCap, isStart bool) {
	dx := seg.end.X - seg.start.X
	dy := seg.end.Y - seg.start.Y
	length := math.Sqrt(dx*dx + dy*dy)
	if length == 0 {
		return
	}

	switch cap {
	case graphics.LineCapRound:
		// Add semicircle
		// Simplified: just add a few line segments
		nx := -dy / length
		ny := dx / length
		if isStart {
			nx, ny = -nx, -ny
		}

		for i := 0; i <= 8; i++ {
			angle := float64(i) * math.Pi / 8
			x := pt.X + halfWidth*(nx*math.Cos(angle)+dx/length*math.Sin(angle))
			y := pt.Y + halfWidth*(ny*math.Cos(angle)+dy/length*math.Sin(angle))
			path.LineTo(x, y)
		}
	case graphics.LineCapSquare:
		// Extend by half width
		tx := dx / length * halfWidth
		ty := dy / length * halfWidth
		if isStart {
			tx, ty = -tx, -ty
		}
		nx := -dy / length * halfWidth
		ny := dx / length * halfWidth

		path.LineTo(pt.X+tx+nx, pt.Y+ty+ny)
		path.LineTo(pt.X+tx-nx, pt.Y+ty-ny)
	case graphics.LineCapButt:
		// Default - no cap needed
	}
}

// DrawLine draws a line between two points.
func (c *Canvas) DrawLine(x1, y1, x2, y2 float64, col color.Color, width float64) {
	path := graphics.NewPath()
	path.MoveTo(x1, y1)
	path.LineTo(x2, y2)
	c.Stroke(path, col, width, graphics.LineCapButt, graphics.LineJoinMiter)
}

// DrawRect draws a rectangle.
func (c *Canvas) DrawRect(x, y, w, h float64, fillColor, strokeColor color.Color, strokeWidth float64) {
	path := graphics.NewPath()
	path.Rect(x, y, w, h)

	if fillColor != nil {
		c.Fill(path, fillColor, graphics.FillRuleNonZero)
	}
	if strokeColor != nil && strokeWidth > 0 {
		c.Stroke(path, strokeColor, strokeWidth, graphics.LineCapButt, graphics.LineJoinMiter)
	}
}

// DrawCircle draws a circle.
func (c *Canvas) DrawCircle(cx, cy, r float64, fillColor, strokeColor color.Color, strokeWidth float64) {
	builder := pathpkg.NewBuilder()
	builder.Circle(cx, cy, r)
	path := builder.Build()

	if fillColor != nil {
		c.Fill(path, fillColor, graphics.FillRuleNonZero)
	}
	if strokeColor != nil && strokeWidth > 0 {
		c.Stroke(path, strokeColor, strokeWidth, graphics.LineCapButt, graphics.LineJoinMiter)
	}
}

// SetPixel sets a single pixel.
func (c *Canvas) SetPixel(x, y int, col color.Color) {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		c.img.Set(x, y, col)
	}
}

// GetPixel gets a pixel color.
func (c *Canvas) GetPixel(x, y int) color.Color {
	if x >= 0 && x < c.width && y >= 0 && y < c.height {
		return c.img.At(x, y)
	}
	return color.Transparent
}

// DrawImage draws an image at the given position.
func (c *Canvas) DrawImage(img image.Image, x, y int) {
	draw.Draw(c.img, image.Rect(x, y, x+img.Bounds().Dx(), y+img.Bounds().Dy()),
		img, image.Point{}, draw.Over)
}

// DrawImageScaled draws an image scaled to fit the given rectangle.
func (c *Canvas) DrawImageScaled(img image.Image, x, y, w, h int) {
	// Simple nearest-neighbor scaling
	srcBounds := img.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			srcX := srcBounds.Min.X + dx*srcW/w
			srcY := srcBounds.Min.Y + dy*srcH/h
			c.img.Set(x+dx, y+dy, img.At(srcX, srcY))
		}
	}
}
