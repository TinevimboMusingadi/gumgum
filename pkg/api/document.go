// Package api provides a clean public API for the PDF renderer.
// This is the main entry point for external consumers.
package api

import (
	"fmt"
	"image"
	"os"

	"gumgum/pkg/cos"
	"gumgum/pkg/raster"
)

// Document represents a PDF document.
type Document struct {
	reader   *cos.Reader
	renderer *raster.Renderer

	// Cached info
	pageCount int
	info      *DocumentInfo
}

// DocumentInfo contains document metadata.
type DocumentInfo struct {
	Title        string
	Author       string
	Subject      string
	Keywords     string
	Creator      string
	Producer     string
	CreationDate string
	ModDate      string
}

// Open opens a PDF file and returns a Document.
func Open(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return OpenBytes(data)
}

// OpenBytes opens a PDF from a byte slice.
func OpenBytes(data []byte) (*Document, error) {
	reader, err := cos.NewReader(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PDF: %w", err)
	}

	pageCount, err := reader.PageCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	doc := &Document{
		reader:    reader,
		renderer:  raster.NewRenderer(reader),
		pageCount: pageCount,
	}

	// Parse document info
	doc.parseInfo()

	return doc, nil
}

// parseInfo extracts document metadata.
func (d *Document) parseInfo() {
	info, err := d.reader.Info()
	if err != nil || info == nil {
		d.info = &DocumentInfo{}
		return
	}

	d.info = &DocumentInfo{
		Title:        getString(info, "Title"),
		Author:       getString(info, "Author"),
		Subject:      getString(info, "Subject"),
		Keywords:     getString(info, "Keywords"),
		Creator:      getString(info, "Creator"),
		Producer:     getString(info, "Producer"),
		CreationDate: getString(info, "CreationDate"),
		ModDate:      getString(info, "ModDate"),
	}
}

func getString(dict cos.Dict, key string) string {
	if val := dict.Get(key); val != nil {
		if s, ok := val.(cos.String); ok {
			return string(s)
		}
	}
	return ""
}

// PageCount returns the number of pages in the document.
func (d *Document) PageCount() int {
	return d.pageCount
}

// Info returns document metadata.
func (d *Document) Info() *DocumentInfo {
	return d.info
}

// Page returns a Page object for the given page number (0-indexed).
func (d *Document) Page(pageNum int) (*Page, error) {
	if pageNum < 0 || pageNum >= d.pageCount {
		return nil, fmt.Errorf("page %d out of range (0-%d)", pageNum, d.pageCount-1)
	}

	pageDict, err := d.reader.GetPage(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get page: %w", err)
	}

	return newPage(d, pageNum, pageDict), nil
}

// Render renders a page to an image with default options.
func (d *Document) Render(pageNum int) (*image.RGBA, error) {
	return d.RenderWithOptions(pageNum, DefaultRenderOptions())
}

// RenderWithOptions renders a page with custom options.
func (d *Document) RenderWithOptions(pageNum int, opts RenderOptions) (*image.RGBA, error) {
	d.renderer.SetDPI(opts.DPI)
	return d.renderer.RenderPage(pageNum)
}

// RenderAllPages renders all pages to images.
func (d *Document) RenderAllPages(opts RenderOptions) ([]*image.RGBA, error) {
	images := make([]*image.RGBA, d.pageCount)

	for i := 0; i < d.pageCount; i++ {
		img, err := d.RenderWithOptions(i, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to render page %d: %w", i, err)
		}
		images[i] = img
	}

	return images, nil
}

// Close releases resources associated with the document.
func (d *Document) Close() error {
	// Currently no cleanup needed, but this provides a consistent API
	return nil
}

// Reader returns the underlying COS reader (for advanced use).
func (d *Document) Reader() *cos.Reader {
	return d.reader
}
