package api

import (
	"image/color"
)

// RenderOptions configures rendering behavior.
type RenderOptions struct {
	// DPI sets the resolution (dots per inch).
	// Default: 150
	DPI float64

	// Scale applies an additional scale factor after DPI.
	// Default: 1.0
	Scale float64

	// Background sets the background color.
	// Default: white
	Background color.Color

	// Transparent enables transparent background (ignores Background).
	// Default: false
	Transparent bool

	// AntiAlias enables anti-aliasing for paths.
	// Default: true
	AntiAlias bool

	// RenderText enables text rendering.
	// Default: true
	RenderText bool

	// RenderImages enables image rendering.
	// Default: true
	RenderImages bool

	// RenderAnnotations enables annotation rendering.
	// Default: true
	RenderAnnotations bool

	// PageRange specifies which pages to render (for batch operations).
	// nil means all pages.
	PageRange *PageRange
}

// PageRange specifies a range of pages.
type PageRange struct {
	Start int // Inclusive, 0-indexed
	End   int // Exclusive
}

// DefaultRenderOptions returns render options with sensible defaults.
func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		DPI:               150,
		Scale:             1.0,
		Background:        color.White,
		Transparent:       false,
		AntiAlias:         true,
		RenderText:        true,
		RenderImages:      true,
		RenderAnnotations: true,
	}
}

// WithDPI returns options with the specified DPI.
func WithDPI(dpi float64) RenderOptions {
	opts := DefaultRenderOptions()
	opts.DPI = dpi
	return opts
}

// WithScale returns options with the specified scale.
func WithScale(scale float64) RenderOptions {
	opts := DefaultRenderOptions()
	opts.Scale = scale
	return opts
}

// WithBackground returns options with the specified background color.
func WithBackground(c color.Color) RenderOptions {
	opts := DefaultRenderOptions()
	opts.Background = c
	return opts
}

// WithTransparent returns options with transparent background.
func WithTransparent() RenderOptions {
	opts := DefaultRenderOptions()
	opts.Transparent = true
	return opts
}

// Option is a functional option for configuring RenderOptions.
type Option func(*RenderOptions)

// DPI sets the resolution.
func DPI(dpi float64) Option {
	return func(o *RenderOptions) {
		o.DPI = dpi
	}
}

// Scale sets the scale factor.
func Scale(scale float64) Option {
	return func(o *RenderOptions) {
		o.Scale = scale
	}
}

// Background sets the background color.
func Background(c color.Color) Option {
	return func(o *RenderOptions) {
		o.Background = c
	}
}

// Transparent enables transparent background.
func Transparent() Option {
	return func(o *RenderOptions) {
		o.Transparent = true
	}
}

// NoText disables text rendering.
func NoText() Option {
	return func(o *RenderOptions) {
		o.RenderText = false
	}
}

// NoImages disables image rendering.
func NoImages() Option {
	return func(o *RenderOptions) {
		o.RenderImages = false
	}
}

// NoAnnotations disables annotation rendering.
func NoAnnotations() Option {
	return func(o *RenderOptions) {
		o.RenderAnnotations = false
	}
}

// NoAntiAlias disables anti-aliasing.
func NoAntiAlias() Option {
	return func(o *RenderOptions) {
		o.AntiAlias = false
	}
}

// Pages sets the page range.
func Pages(start, end int) Option {
	return func(o *RenderOptions) {
		o.PageRange = &PageRange{Start: start, End: end}
	}
}

// NewRenderOptions creates options from functional options.
func NewRenderOptions(opts ...Option) RenderOptions {
	o := DefaultRenderOptions()
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// Apply applies functional options to existing options.
func (o *RenderOptions) Apply(opts ...Option) {
	for _, opt := range opts {
		opt(o)
	}
}

// EffectiveDPI returns the final DPI after applying scale.
func (o *RenderOptions) EffectiveDPI() float64 {
	return o.DPI * o.Scale
}

// Export options for saving rendered pages.
type ExportOptions struct {
	// Format specifies the output format: "png", "jpeg", "gif"
	Format string

	// Quality for JPEG (1-100)
	Quality int

	// Compression for PNG (0-9, where 0 is no compression)
	Compression int
}

// DefaultExportOptions returns default export options.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Format:      "png",
		Quality:     90,
		Compression: 6,
	}
}

// PNG returns export options for PNG format.
func PNG() ExportOptions {
	return ExportOptions{
		Format:      "png",
		Compression: 6,
	}
}

// JPEG returns export options for JPEG format with quality.
func JPEG(quality int) ExportOptions {
	if quality < 1 {
		quality = 1
	}
	if quality > 100 {
		quality = 100
	}
	return ExportOptions{
		Format:  "jpeg",
		Quality: quality,
	}
}
