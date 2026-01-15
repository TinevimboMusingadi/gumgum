package api

import (
	"image"

	"gumgum/pkg/cos"
)

// Page represents a single page in a PDF document.
type Page struct {
	doc      *Document
	pageNum  int
	dict     cos.Dict
	size     PageSize
	rotation int
}

// PageSize contains page dimensions.
type PageSize struct {
	Width  float64 // Width in points (1/72 inch)
	Height float64 // Height in points
}

// Common page sizes in points
var (
	PageSizeLetter = PageSize{612, 792}
	PageSizeA4     = PageSize{595.28, 841.89}
	PageSizeA3     = PageSize{841.89, 1190.55}
	PageSizeLegal  = PageSize{612, 1008}
)

// newPage creates a new Page object.
func newPage(doc *Document, pageNum int, dict cos.Dict) *Page {
	p := &Page{
		doc:     doc,
		pageNum: pageNum,
		dict:    dict,
		size:    PageSizeLetter, // Default
	}

	// Parse MediaBox
	if mediaBox, ok := dict.GetArray("MediaBox"); ok && len(mediaBox) >= 4 {
		x1 := toFloat(mediaBox[0])
		y1 := toFloat(mediaBox[1])
		x2 := toFloat(mediaBox[2])
		y2 := toFloat(mediaBox[3])
		p.size = PageSize{
			Width:  x2 - x1,
			Height: y2 - y1,
		}
	}

	// Parse Rotation
	if rot, ok := dict.GetInt("Rotate"); ok {
		p.rotation = int(rot)
	}

	return p
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

// Number returns the page number (0-indexed).
func (p *Page) Number() int {
	return p.pageNum
}

// Size returns the page dimensions in points.
func (p *Page) Size() PageSize {
	return p.size
}

// Width returns the page width in points.
func (p *Page) Width() float64 {
	return p.size.Width
}

// Height returns the page height in points.
func (p *Page) Height() float64 {
	return p.size.Height
}

// Rotation returns the page rotation in degrees (0, 90, 180, 270).
func (p *Page) Rotation() int {
	return p.rotation
}

// AspectRatio returns the width/height ratio.
func (p *Page) AspectRatio() float64 {
	if p.size.Height == 0 {
		return 1
	}
	return p.size.Width / p.size.Height
}

// IsLandscape returns true if width > height.
func (p *Page) IsLandscape() bool {
	return p.size.Width > p.size.Height
}

// Render renders the page with default options.
func (p *Page) Render() (*image.RGBA, error) {
	return p.RenderWithOptions(DefaultRenderOptions())
}

// RenderWithOptions renders the page with custom options.
func (p *Page) RenderWithOptions(opts RenderOptions) (*image.RGBA, error) {
	return p.doc.RenderWithOptions(p.pageNum, opts)
}

// SizeInPixels returns the page size in pixels at the given DPI.
func (p *Page) SizeInPixels(dpi float64) (width, height int) {
	width = int(p.size.Width * dpi / 72)
	height = int(p.size.Height * dpi / 72)
	return
}

// Contents returns the raw decoded content stream.
func (p *Page) Contents() ([]byte, error) {
	return p.doc.reader.GetPageContents(p.dict)
}

// Dict returns the underlying page dictionary (for advanced use).
func (p *Page) Dict() cos.Dict {
	return p.dict
}

// CropBox returns the crop box if set, otherwise the media box.
func (p *Page) CropBox() (x1, y1, x2, y2 float64) {
	// Try CropBox first
	if cropBox, ok := p.dict.GetArray("CropBox"); ok && len(cropBox) >= 4 {
		return toFloat(cropBox[0]), toFloat(cropBox[1]),
			toFloat(cropBox[2]), toFloat(cropBox[3])
	}

	// Fall back to MediaBox
	if mediaBox, ok := p.dict.GetArray("MediaBox"); ok && len(mediaBox) >= 4 {
		return toFloat(mediaBox[0]), toFloat(mediaBox[1]),
			toFloat(mediaBox[2]), toFloat(mediaBox[3])
	}

	// Default to Letter
	return 0, 0, 612, 792
}

// BleedBox returns the bleed box if set.
func (p *Page) BleedBox() (x1, y1, x2, y2 float64, ok bool) {
	if box, exists := p.dict.GetArray("BleedBox"); exists && len(box) >= 4 {
		return toFloat(box[0]), toFloat(box[1]),
			toFloat(box[2]), toFloat(box[3]), true
	}
	return 0, 0, 0, 0, false
}

// TrimBox returns the trim box if set.
func (p *Page) TrimBox() (x1, y1, x2, y2 float64, ok bool) {
	if box, exists := p.dict.GetArray("TrimBox"); exists && len(box) >= 4 {
		return toFloat(box[0]), toFloat(box[1]),
			toFloat(box[2]), toFloat(box[3]), true
	}
	return 0, 0, 0, 0, false
}

// ArtBox returns the art box if set.
func (p *Page) ArtBox() (x1, y1, x2, y2 float64, ok bool) {
	if box, exists := p.dict.GetArray("ArtBox"); exists && len(box) >= 4 {
		return toFloat(box[0]), toFloat(box[1]),
			toFloat(box[2]), toFloat(box[3]), true
	}
	return 0, 0, 0, 0, false
}
