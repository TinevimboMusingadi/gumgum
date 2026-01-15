// Package path provides path construction utilities for the rasterizer.
package path

import (
	"math"

	"gumgum/pkg/graphics"

	"golang.org/x/image/vector"
)

// ToVector converts a graphics.Path to a golang.org/x/image/vector path.
func ToVector(p *graphics.Path, rasterizer *vector.Rasterizer) {
	for _, seg := range p.Segments {
		switch seg.Op {
		case graphics.PathOpMoveTo:
			if len(seg.Points) >= 1 {
				rasterizer.MoveTo(
					float32(seg.Points[0].X),
					float32(seg.Points[0].Y),
				)
			}
		case graphics.PathOpLineTo:
			if len(seg.Points) >= 1 {
				rasterizer.LineTo(
					float32(seg.Points[0].X),
					float32(seg.Points[0].Y),
				)
			}
		case graphics.PathOpCurveTo:
			if len(seg.Points) >= 3 {
				rasterizer.CubeTo(
					float32(seg.Points[0].X), float32(seg.Points[0].Y),
					float32(seg.Points[1].X), float32(seg.Points[1].Y),
					float32(seg.Points[2].X), float32(seg.Points[2].Y),
				)
			}
		case graphics.PathOpClose:
			rasterizer.ClosePath()
		}
	}
}

// Builder provides a fluent interface for building paths.
type Builder struct {
	path *graphics.Path
}

// NewBuilder creates a new path builder.
func NewBuilder() *Builder {
	return &Builder{
		path: graphics.NewPath(),
	}
}

// MoveTo starts a new subpath.
func (b *Builder) MoveTo(x, y float64) *Builder {
	b.path.MoveTo(x, y)
	return b
}

// LineTo draws a line to the given point.
func (b *Builder) LineTo(x, y float64) *Builder {
	b.path.LineTo(x, y)
	return b
}

// CurveTo draws a cubic Bezier curve.
func (b *Builder) CurveTo(cp1x, cp1y, cp2x, cp2y, x, y float64) *Builder {
	b.path.CurveTo(cp1x, cp1y, cp2x, cp2y, x, y)
	return b
}

// QuadTo draws a quadratic Bezier curve (converted to cubic).
func (b *Builder) QuadTo(cpx, cpy, x, y float64) *Builder {
	// Convert quadratic to cubic
	// Current point
	cur := b.path.CurrentPoint()

	// Control points for cubic
	cp1x := cur.X + 2.0/3.0*(cpx-cur.X)
	cp1y := cur.Y + 2.0/3.0*(cpy-cur.Y)
	cp2x := x + 2.0/3.0*(cpx-x)
	cp2y := y + 2.0/3.0*(cpy-y)

	b.path.CurveTo(cp1x, cp1y, cp2x, cp2y, x, y)
	return b
}

// Close closes the current subpath.
func (b *Builder) Close() *Builder {
	b.path.Close()
	return b
}

// Rect adds a rectangle to the path.
func (b *Builder) Rect(x, y, w, h float64) *Builder {
	b.path.Rect(x, y, w, h)
	return b
}

// RoundRect adds a rounded rectangle to the path.
func (b *Builder) RoundRect(x, y, w, h, rx, ry float64) *Builder {
	// Clamp radii
	if rx > w/2 {
		rx = w / 2
	}
	if ry > h/2 {
		ry = h / 2
	}

	// Magic number for cubic bezier approximation of quarter circle
	k := 0.5522847498307936

	b.MoveTo(x+rx, y)
	b.LineTo(x+w-rx, y)
	b.CurveTo(x+w-rx+rx*k, y, x+w, y+ry-ry*k, x+w, y+ry)
	b.LineTo(x+w, y+h-ry)
	b.CurveTo(x+w, y+h-ry+ry*k, x+w-rx+rx*k, y+h, x+w-rx, y+h)
	b.LineTo(x+rx, y+h)
	b.CurveTo(x+rx-rx*k, y+h, x, y+h-ry+ry*k, x, y+h-ry)
	b.LineTo(x, y+ry)
	b.CurveTo(x, y+ry-ry*k, x+rx-rx*k, y, x+rx, y)
	b.Close()

	return b
}

// Circle adds a circle to the path.
func (b *Builder) Circle(cx, cy, r float64) *Builder {
	return b.Ellipse(cx, cy, r, r)
}

// Ellipse adds an ellipse to the path.
func (b *Builder) Ellipse(cx, cy, rx, ry float64) *Builder {
	k := 0.5522847498307936

	b.MoveTo(cx+rx, cy)
	b.CurveTo(cx+rx, cy+ry*k, cx+rx*k, cy+ry, cx, cy+ry)
	b.CurveTo(cx-rx*k, cy+ry, cx-rx, cy+ry*k, cx-rx, cy)
	b.CurveTo(cx-rx, cy-ry*k, cx-rx*k, cy-ry, cx, cy-ry)
	b.CurveTo(cx+rx*k, cy-ry, cx+rx, cy-ry*k, cx+rx, cy)
	b.Close()

	return b
}

// Arc adds an arc to the path.
func (b *Builder) Arc(cx, cy, r, startAngle, endAngle float64) *Builder {
	segments := int((endAngle - startAngle) / (math.Pi / 4))
	if segments < 1 {
		segments = 1
	}

	angleStep := (endAngle - startAngle) / float64(segments)

	// Start point
	x := cx + r*math.Cos(startAngle)
	y := cy + r*math.Sin(startAngle)
	b.MoveTo(x, y)

	for i := 0; i < segments; i++ {
		a1 := startAngle + float64(i)*angleStep
		a2 := a1 + angleStep

		// Approximate arc with cubic bezier
		x2 := cx + r*math.Cos(a2)
		y2 := cy + r*math.Sin(a2)

		// Control points
		k := 4.0 / 3.0 * math.Tan(angleStep/4)
		cp1x := x - k*r*math.Sin(a1)
		cp1y := y + k*r*math.Cos(a1)
		cp2x := x2 + k*r*math.Sin(a2)
		cp2y := y2 - k*r*math.Cos(a2)

		b.CurveTo(cp1x, cp1y, cp2x, cp2y, x2, y2)

		x, y = x2, y2
	}

	return b
}

// Build returns the constructed path.
func (b *Builder) Build() *graphics.Path {
	return b.path
}

// Clear resets the builder for reuse.
func (b *Builder) Clear() *Builder {
	b.path.Clear()
	return b
}
