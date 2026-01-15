package raster

import (
	"fmt"
	"image"
	"image/png"
	"os"

	"gumgum/pkg/cos"
	"gumgum/pkg/graphics"
)

// Renderer renders PDF pages to images.
type Renderer struct {
	reader *cos.Reader
	dpi    float64
}

// NewRenderer creates a new renderer for a PDF reader.
func NewRenderer(reader *cos.Reader) *Renderer {
	return &Renderer{
		reader: reader,
		dpi:    150, // Default DPI
	}
}

// SetDPI sets the rendering DPI.
func (r *Renderer) SetDPI(dpi float64) {
	r.dpi = dpi
}

// RenderPage renders a page to an image.
func (r *Renderer) RenderPage(pageNum int) (*image.RGBA, error) {
	// Get page
	page, err := r.reader.GetPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}

	// Get page dimensions from MediaBox
	var width, height float64 = 612, 792 // Default to US Letter

	if mediaBox, ok := page.GetArray("MediaBox"); ok && len(mediaBox) >= 4 {
		x1 := toFloat(mediaBox[0])
		y1 := toFloat(mediaBox[1])
		x2 := toFloat(mediaBox[2])
		y2 := toFloat(mediaBox[3])
		width = x2 - x1
		height = y2 - y1
	}

	// Create canvas
	canvas := NewCanvasWithDPI(width, height, r.dpi)
	canvas.Clear()

	// Get page contents
	contents, err := r.reader.GetPageContents(page)
	if err != nil {
		return canvas.Image(), fmt.Errorf("failed to get page contents: %w", err)
	}

	if len(contents) == 0 {
		return canvas.Image(), nil
	}

	// Parse content stream
	ops, err := graphics.ParseContentStream(contents)
	if err != nil {
		return canvas.Image(), fmt.Errorf("failed to parse content stream: %w", err)
	}

	// Create interpreter
	interp := graphics.NewInterpreter()

	// Scale factor for DPI
	scale := r.dpi / 72.0

	// Set up rendering callbacks
	interp.OnFill = func(path *graphics.Path, state *graphics.State, rule graphics.FillRule) {
		// Transform path for rendering (flip Y and scale)
		transformed := transformPath(path, height, scale)
		col := state.FillColor.WithAlpha(state.FillAlpha)
		canvas.Fill(transformed, col, rule)
	}

	interp.OnStroke = func(path *graphics.Path, state *graphics.State) {
		transformed := transformPath(path, height, scale)
		col := state.StrokeColor.WithAlpha(state.StrokeAlpha)
		lineWidth := state.LineWidth * scale
		if lineWidth < 1 {
			lineWidth = 1
		}
		canvas.Stroke(transformed, col, lineWidth, state.LineCap, state.LineJoin)
	}

	interp.OnText = func(text string, state *graphics.State) {
		// Text rendering will be handled by the font package
		// For now, this is a placeholder
		_ = text
	}

	interp.OnImage = func(name string, state *graphics.State) {
		// Image rendering will be handled later
		_ = name
	}

	// Execute operators
	if err := interp.Execute(ops); err != nil {
		// Log but don't fail
		fmt.Printf("Warning: execution error: %v\n", err)
	}

	return canvas.Image(), nil
}

// transformPath transforms a path from PDF coordinates to image coordinates.
// PDF has origin at bottom-left, images have origin at top-left.
func transformPath(path *graphics.Path, pageHeight, scale float64) *graphics.Path {
	result := graphics.NewPath()

	for _, seg := range path.Segments {
		switch seg.Op {
		case graphics.PathOpMoveTo:
			if len(seg.Points) > 0 {
				x, y := transformPoint(seg.Points[0].X, seg.Points[0].Y, pageHeight, scale)
				result.MoveTo(x, y)
			}
		case graphics.PathOpLineTo:
			if len(seg.Points) > 0 {
				x, y := transformPoint(seg.Points[0].X, seg.Points[0].Y, pageHeight, scale)
				result.LineTo(x, y)
			}
		case graphics.PathOpCurveTo:
			if len(seg.Points) >= 3 {
				x1, y1 := transformPoint(seg.Points[0].X, seg.Points[0].Y, pageHeight, scale)
				x2, y2 := transformPoint(seg.Points[1].X, seg.Points[1].Y, pageHeight, scale)
				x3, y3 := transformPoint(seg.Points[2].X, seg.Points[2].Y, pageHeight, scale)
				result.CurveTo(x1, y1, x2, y2, x3, y3)
			}
		case graphics.PathOpClose:
			result.Close()
		}
	}

	return result
}

// transformPoint converts PDF coordinates to image coordinates.
func transformPoint(x, y, pageHeight, scale float64) (float64, float64) {
	return x * scale, (pageHeight - y) * scale
}

func toFloat(obj cos.Object) float64 {
	switch v := obj.(type) {
	case cos.Integer:
		return float64(v)
	case cos.Real:
		return float64(v)
	}
	return 0
}

// RenderToFile renders a page and saves it to a file.
func (r *Renderer) RenderToFile(pageNum int, filename string) error {
	img, err := r.RenderPage(pageNum)
	if err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer f.Close()

	return png.Encode(f, img)
}

// RenderAllPages renders all pages to a slice of images.
func (r *Renderer) RenderAllPages() ([]*image.RGBA, error) {
	count, err := r.reader.PageCount()
	if err != nil {
		return nil, err
	}

	images := make([]*image.RGBA, count)
	for i := 0; i < count; i++ {
		img, err := r.RenderPage(i)
		if err != nil {
			return nil, fmt.Errorf("failed to render page %d: %w", i, err)
		}
		images[i] = img
	}

	return images, nil
}
