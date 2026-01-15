// Package graphics implements the PDF graphics state machine.
// This interprets the drawing commands in content streams and
// maintains the current graphics state (colors, transforms, etc.)
package graphics

import (
	"math"
)

// Matrix represents a 3x3 affine transformation matrix.
// Only the first two rows are stored since the third row is always [0 0 1].
// The matrix is stored as:
//   [A B 0]
//   [C D 0]
//   [E F 1]
// Where (A,B,C,D) handle scaling/rotation and (E,F) handle translation.
type Matrix [6]float64

// Identity returns the identity matrix.
func Identity() Matrix {
	return Matrix{1, 0, 0, 1, 0, 0}
}

// Translate returns a translation matrix.
func Translate(tx, ty float64) Matrix {
	return Matrix{1, 0, 0, 1, tx, ty}
}

// Scale returns a scaling matrix.
func Scale(sx, sy float64) Matrix {
	return Matrix{sx, 0, 0, sy, 0, 0}
}

// Rotate returns a rotation matrix (angle in radians).
func Rotate(angle float64) Matrix {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return Matrix{cos, sin, -sin, cos, 0, 0}
}

// RotateDeg returns a rotation matrix (angle in degrees).
func RotateDeg(angle float64) Matrix {
	return Rotate(angle * math.Pi / 180)
}

// Skew returns a skew matrix (angles in radians).
func Skew(angleX, angleY float64) Matrix {
	return Matrix{1, math.Tan(angleY), math.Tan(angleX), 1, 0, 0}
}

// Multiply multiplies two matrices: result = m * other
func (m Matrix) Multiply(other Matrix) Matrix {
	return Matrix{
		m[0]*other[0] + m[1]*other[2],
		m[0]*other[1] + m[1]*other[3],
		m[2]*other[0] + m[3]*other[2],
		m[2]*other[1] + m[3]*other[3],
		m[4]*other[0] + m[5]*other[2] + other[4],
		m[4]*other[1] + m[5]*other[3] + other[5],
	}
}

// Concat concatenates another matrix to this one: m = other * m
func (m *Matrix) Concat(other Matrix) {
	*m = other.Multiply(*m)
}

// Transform applies the matrix to a point.
func (m Matrix) Transform(x, y float64) (float64, float64) {
	return m[0]*x + m[2]*y + m[4], m[1]*x + m[3]*y + m[5]
}

// TransformPoint applies the matrix to a Point.
func (m Matrix) TransformPoint(p Point) Point {
	x, y := m.Transform(p.X, p.Y)
	return Point{x, y}
}

// TransformVector applies the matrix to a vector (without translation).
func (m Matrix) TransformVector(dx, dy float64) (float64, float64) {
	return m[0]*dx + m[2]*dy, m[1]*dx + m[3]*dy
}

// Determinant returns the determinant of the matrix.
func (m Matrix) Determinant() float64 {
	return m[0]*m[3] - m[1]*m[2]
}

// Inverse returns the inverse of the matrix.
func (m Matrix) Inverse() Matrix {
	det := m.Determinant()
	if det == 0 {
		return Identity()
	}
	
	return Matrix{
		m[3] / det,
		-m[1] / det,
		-m[2] / det,
		m[0] / det,
		(m[2]*m[5] - m[3]*m[4]) / det,
		(m[1]*m[4] - m[0]*m[5]) / det,
	}
}

// ScaleX returns the horizontal scaling factor.
func (m Matrix) ScaleX() float64 {
	return math.Sqrt(m[0]*m[0] + m[1]*m[1])
}

// ScaleY returns the vertical scaling factor.
func (m Matrix) ScaleY() float64 {
	return math.Sqrt(m[2]*m[2] + m[3]*m[3])
}

// Rotation returns the rotation angle in radians.
func (m Matrix) Rotation() float64 {
	return math.Atan2(m[1], m[0])
}

// Point represents a 2D point.
type Point struct {
	X, Y float64
}

// Add returns the sum of two points (vector addition).
func (p Point) Add(other Point) Point {
	return Point{p.X + other.X, p.Y + other.Y}
}

// Sub returns the difference of two points.
func (p Point) Sub(other Point) Point {
	return Point{p.X - other.X, p.Y - other.Y}
}

// Scale scales the point by a factor.
func (p Point) Scale(s float64) Point {
	return Point{p.X * s, p.Y * s}
}

// Length returns the distance from origin.
func (p Point) Length() float64 {
	return math.Sqrt(p.X*p.X + p.Y*p.Y)
}

// Normalize returns a unit vector in the same direction.
func (p Point) Normalize() Point {
	l := p.Length()
	if l == 0 {
		return Point{}
	}
	return Point{p.X / l, p.Y / l}
}

// Rect represents a rectangle.
type Rect struct {
	X, Y, Width, Height float64
}

// NewRect creates a rectangle from two corner points.
func NewRect(x1, y1, x2, y2 float64) Rect {
	if x1 > x2 {
		x1, x2 = x2, x1
	}
	if y1 > y2 {
		y1, y2 = y2, y1
	}
	return Rect{
		X:      x1,
		Y:      y1,
		Width:  x2 - x1,
		Height: y2 - y1,
	}
}

// Contains returns true if the point is inside the rectangle.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X <= r.X+r.Width &&
		p.Y >= r.Y && p.Y <= r.Y+r.Height
}

// Intersects returns true if this rectangle intersects another.
func (r Rect) Intersects(other Rect) bool {
	return r.X < other.X+other.Width && r.X+r.Width > other.X &&
		r.Y < other.Y+other.Height && r.Y+r.Height > other.Y
}

// Union returns the smallest rectangle containing both rectangles.
func (r Rect) Union(other Rect) Rect {
	x1 := math.Min(r.X, other.X)
	y1 := math.Min(r.Y, other.Y)
	x2 := math.Max(r.X+r.Width, other.X+other.Width)
	y2 := math.Max(r.Y+r.Height, other.Y+other.Height)
	return NewRect(x1, y1, x2, y2)
}

// Transform applies a matrix transformation to the rectangle.
func (r Rect) Transform(m Matrix) Rect {
	// Transform all four corners and find bounding box
	corners := []Point{
		{r.X, r.Y},
		{r.X + r.Width, r.Y},
		{r.X + r.Width, r.Y + r.Height},
		{r.X, r.Y + r.Height},
	}
	
	transformed := make([]Point, 4)
	for i, c := range corners {
		transformed[i] = m.TransformPoint(c)
	}
	
	minX := transformed[0].X
	maxX := transformed[0].X
	minY := transformed[0].Y
	maxY := transformed[0].Y
	
	for _, p := range transformed[1:] {
		minX = math.Min(minX, p.X)
		maxX = math.Max(maxX, p.X)
		minY = math.Min(minY, p.Y)
		maxY = math.Max(maxY, p.Y)
	}
	
	return NewRect(minX, minY, maxX, maxY)
}
