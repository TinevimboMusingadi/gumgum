package graphics

import (
	"math"
)

// PathOp represents a path operation type.
type PathOp int

const (
	PathOpMoveTo PathOp = iota
	PathOpLineTo
	PathOpCurveTo // Cubic bezier
	PathOpClose
)

// PathSegment represents a single segment in a path.
type PathSegment struct {
	Op     PathOp
	Points []Point
}

// Path represents a graphics path (a sequence of connected lines and curves).
type Path struct {
	Segments []PathSegment
	current  Point
	start    Point // Start of current subpath
}

// NewPath creates a new empty path.
func NewPath() *Path {
	return &Path{}
}

// Clone creates a deep copy of the path.
func (p *Path) Clone() *Path {
	clone := &Path{
		Segments: make([]PathSegment, len(p.Segments)),
		current:  p.current,
		start:    p.start,
	}
	for i, seg := range p.Segments {
		clone.Segments[i] = PathSegment{
			Op:     seg.Op,
			Points: make([]Point, len(seg.Points)),
		}
		copy(clone.Segments[i].Points, seg.Points)
	}
	return clone
}

// MoveTo starts a new subpath at the given point.
func (p *Path) MoveTo(x, y float64) {
	pt := Point{x, y}
	p.Segments = append(p.Segments, PathSegment{
		Op:     PathOpMoveTo,
		Points: []Point{pt},
	})
	p.current = pt
	p.start = pt
}

// LineTo draws a line from the current point to the given point.
func (p *Path) LineTo(x, y float64) {
	pt := Point{x, y}
	p.Segments = append(p.Segments, PathSegment{
		Op:     PathOpLineTo,
		Points: []Point{pt},
	})
	p.current = pt
}

// CurveTo draws a cubic Bezier curve from the current point.
// cp1 and cp2 are control points, end is the endpoint.
func (p *Path) CurveTo(cp1x, cp1y, cp2x, cp2y, endX, endY float64) {
	p.Segments = append(p.Segments, PathSegment{
		Op: PathOpCurveTo,
		Points: []Point{
			{cp1x, cp1y},
			{cp2x, cp2y},
			{endX, endY},
		},
	})
	p.current = Point{endX, endY}
}

// CurveToV draws a cubic Bezier curve with the first control point at the current point.
// (PDF 'v' operator)
func (p *Path) CurveToV(cp2x, cp2y, endX, endY float64) {
	p.CurveTo(p.current.X, p.current.Y, cp2x, cp2y, endX, endY)
}

// CurveToY draws a cubic Bezier curve with the second control point at the endpoint.
// (PDF 'y' operator)
func (p *Path) CurveToY(cp1x, cp1y, endX, endY float64) {
	p.CurveTo(cp1x, cp1y, endX, endY, endX, endY)
}

// Close closes the current subpath with a line back to the start.
func (p *Path) Close() {
	p.Segments = append(p.Segments, PathSegment{
		Op: PathOpClose,
	})
	p.current = p.start
}

// Rect adds a rectangle to the path.
func (p *Path) Rect(x, y, width, height float64) {
	p.MoveTo(x, y)
	p.LineTo(x+width, y)
	p.LineTo(x+width, y+height)
	p.LineTo(x, y+height)
	p.Close()
}

// Clear removes all segments from the path.
func (p *Path) Clear() {
	p.Segments = p.Segments[:0]
	p.current = Point{}
	p.start = Point{}
}

// IsEmpty returns true if the path has no segments.
func (p *Path) IsEmpty() bool {
	return len(p.Segments) == 0
}

// CurrentPoint returns the current point.
func (p *Path) CurrentPoint() Point {
	return p.current
}

// Bounds returns the bounding box of the path.
func (p *Path) Bounds() Rect {
	if len(p.Segments) == 0 {
		return Rect{}
	}
	
	minX := math.MaxFloat64
	minY := math.MaxFloat64
	maxX := -math.MaxFloat64
	maxY := -math.MaxFloat64
	
	for _, seg := range p.Segments {
		for _, pt := range seg.Points {
			minX = math.Min(minX, pt.X)
			minY = math.Min(minY, pt.Y)
			maxX = math.Max(maxX, pt.X)
			maxY = math.Max(maxY, pt.Y)
		}
	}
	
	if minX == math.MaxFloat64 {
		return Rect{}
	}
	
	return NewRect(minX, minY, maxX, maxY)
}

// Transform applies a transformation matrix to all points in the path.
func (p *Path) Transform(m Matrix) *Path {
	result := NewPath()
	for _, seg := range p.Segments {
		newSeg := PathSegment{
			Op:     seg.Op,
			Points: make([]Point, len(seg.Points)),
		}
		for i, pt := range seg.Points {
			newSeg.Points[i] = m.TransformPoint(pt)
		}
		result.Segments = append(result.Segments, newSeg)
	}
	if len(p.Segments) > 0 {
		result.current = m.TransformPoint(p.current)
		result.start = m.TransformPoint(p.start)
	}
	return result
}

// FillRule represents the fill rule for path filling.
type FillRule int

const (
	FillRuleNonZero FillRule = iota
	FillRuleEvenOdd
)

// Contains checks if a point is inside the path using the specified fill rule.
func (p *Path) Contains(pt Point, rule FillRule) bool {
	if rule == FillRuleEvenOdd {
		return p.containsEvenOdd(pt)
	}
	return p.containsNonZero(pt)
}

// containsNonZero implements the non-zero winding rule.
func (p *Path) containsNonZero(pt Point) bool {
	winding := 0
	var prevPt Point
	var startPt Point
	
	for _, seg := range p.Segments {
		switch seg.Op {
		case PathOpMoveTo:
			if len(seg.Points) > 0 {
				prevPt = seg.Points[0]
				startPt = prevPt
			}
		case PathOpLineTo:
			if len(seg.Points) > 0 {
				endPt := seg.Points[0]
				winding += windingLine(pt, prevPt, endPt)
				prevPt = endPt
			}
		case PathOpCurveTo:
			// Approximate curve with line segments for containment test
			if len(seg.Points) >= 3 {
				endPt := seg.Points[2]
				winding += windingLine(pt, prevPt, endPt)
				prevPt = endPt
			}
		case PathOpClose:
			winding += windingLine(pt, prevPt, startPt)
			prevPt = startPt
		}
	}
	
	return winding != 0
}

// containsEvenOdd implements the even-odd fill rule.
func (p *Path) containsEvenOdd(pt Point) bool {
	crossings := 0
	var prevPt Point
	var startPt Point
	
	for _, seg := range p.Segments {
		switch seg.Op {
		case PathOpMoveTo:
			if len(seg.Points) > 0 {
				prevPt = seg.Points[0]
				startPt = prevPt
			}
		case PathOpLineTo:
			if len(seg.Points) > 0 {
				endPt := seg.Points[0]
				if rayIntersectsLine(pt, prevPt, endPt) {
					crossings++
				}
				prevPt = endPt
			}
		case PathOpCurveTo:
			if len(seg.Points) >= 3 {
				endPt := seg.Points[2]
				if rayIntersectsLine(pt, prevPt, endPt) {
					crossings++
				}
				prevPt = endPt
			}
		case PathOpClose:
			if rayIntersectsLine(pt, prevPt, startPt) {
				crossings++
			}
			prevPt = startPt
		}
	}
	
	return crossings%2 == 1
}

// windingLine returns the winding contribution of a line segment.
func windingLine(pt, p1, p2 Point) int {
	if p1.Y <= pt.Y {
		if p2.Y > pt.Y {
			if isLeft(p1, p2, pt) > 0 {
				return 1
			}
		}
	} else {
		if p2.Y <= pt.Y {
			if isLeft(p1, p2, pt) < 0 {
				return -1
			}
		}
	}
	return 0
}

// isLeft returns a value indicating which side of a line a point is on.
func isLeft(p0, p1, p2 Point) float64 {
	return (p1.X-p0.X)*(p2.Y-p0.Y) - (p2.X-p0.X)*(p1.Y-p0.Y)
}

// rayIntersectsLine tests if a horizontal ray from pt intersects the line segment.
func rayIntersectsLine(pt, p1, p2 Point) bool {
	if (p1.Y > pt.Y) == (p2.Y > pt.Y) {
		return false
	}
	
	t := (pt.Y - p1.Y) / (p2.Y - p1.Y)
	x := p1.X + t*(p2.X-p1.X)
	
	return x > pt.X
}
