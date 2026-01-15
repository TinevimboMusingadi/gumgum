package graphics

import (
	"image/color"
	"math"
)

// ColorSpace represents a PDF color space.
type ColorSpace string

const (
	ColorSpaceDeviceGray ColorSpace = "DeviceGray"
	ColorSpaceDeviceRGB  ColorSpace = "DeviceRGB"
	ColorSpaceCMYK       ColorSpace = "DeviceCMYK"
	ColorSpacePattern    ColorSpace = "Pattern"
	ColorSpaceSeparation ColorSpace = "Separation"
	ColorSpaceIndexed    ColorSpace = "Indexed"
	ColorSpaceLab        ColorSpace = "Lab"
	ColorSpaceICCBased   ColorSpace = "ICCBased"
)

// Color represents a PDF color value.
type Color struct {
	Space      ColorSpace
	Components []float64
}

// NewGray creates a grayscale color.
func NewGray(gray float64) Color {
	return Color{
		Space:      ColorSpaceDeviceGray,
		Components: []float64{clamp(gray, 0, 1)},
	}
}

// NewRGB creates an RGB color.
func NewRGB(r, g, b float64) Color {
	return Color{
		Space:      ColorSpaceDeviceRGB,
		Components: []float64{clamp(r, 0, 1), clamp(g, 0, 1), clamp(b, 0, 1)},
	}
}

// NewCMYK creates a CMYK color.
func NewCMYK(c, m, y, k float64) Color {
	return Color{
		Space:      ColorSpaceCMYK,
		Components: []float64{clamp(c, 0, 1), clamp(m, 0, 1), clamp(y, 0, 1), clamp(k, 0, 1)},
	}
}

// Black returns a black color.
func Black() Color {
	return NewGray(0)
}

// White returns a white color.
func White() Color {
	return NewGray(1)
}

// ToRGBA converts the color to RGBA.
func (c Color) ToRGBA() color.RGBA {
	switch c.Space {
	case ColorSpaceDeviceGray:
		if len(c.Components) >= 1 {
			g := uint8(c.Components[0] * 255)
			return color.RGBA{g, g, g, 255}
		}
	case ColorSpaceDeviceRGB:
		if len(c.Components) >= 3 {
			return color.RGBA{
				uint8(c.Components[0] * 255),
				uint8(c.Components[1] * 255),
				uint8(c.Components[2] * 255),
				255,
			}
		}
	case ColorSpaceCMYK:
		if len(c.Components) >= 4 {
			r, g, b := cmykToRGB(c.Components[0], c.Components[1], c.Components[2], c.Components[3])
			return color.RGBA{
				uint8(r * 255),
				uint8(g * 255),
				uint8(b * 255),
				255,
			}
		}
	}
	
	// Default to black
	return color.RGBA{0, 0, 0, 255}
}

// cmykToRGB converts CMYK to RGB.
func cmykToRGB(c, m, y, k float64) (r, g, b float64) {
	r = (1 - c) * (1 - k)
	g = (1 - m) * (1 - k)
	b = (1 - y) * (1 - k)
	return
}

// WithAlpha returns a new color with the given alpha.
func (c Color) WithAlpha(alpha float64) color.NRGBA {
	rgba := c.ToRGBA()
	return color.NRGBA{
		R: rgba.R,
		G: rgba.G,
		B: rgba.B,
		A: uint8(clamp(alpha, 0, 1) * 255),
	}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// LineCap represents the line cap style.
type LineCap int

const (
	LineCapButt   LineCap = 0
	LineCapRound  LineCap = 1
	LineCapSquare LineCap = 2
)

// LineJoin represents the line join style.
type LineJoin int

const (
	LineJoinMiter LineJoin = 0
	LineJoinRound LineJoin = 1
	LineJoinBevel LineJoin = 2
)

// BlendMode represents the blend mode for compositing.
type BlendMode string

const (
	BlendNormal     BlendMode = "Normal"
	BlendMultiply   BlendMode = "Multiply"
	BlendScreen     BlendMode = "Screen"
	BlendOverlay    BlendMode = "Overlay"
	BlendDarken     BlendMode = "Darken"
	BlendLighten    BlendMode = "Lighten"
	BlendColorDodge BlendMode = "ColorDodge"
	BlendColorBurn  BlendMode = "ColorBurn"
	BlendHardLight  BlendMode = "HardLight"
	BlendSoftLight  BlendMode = "SoftLight"
	BlendDifference BlendMode = "Difference"
	BlendExclusion  BlendMode = "Exclusion"
)

// Blend applies a blend mode to two colors.
func Blend(mode BlendMode, backdrop, source Color) Color {
	// Convert both to RGB for blending
	br := backdrop.ToRGBA()
	sr := source.ToRGBA()
	
	var r, g, b float64
	bR := float64(br.R) / 255
	bG := float64(br.G) / 255
	bB := float64(br.B) / 255
	sR := float64(sr.R) / 255
	sG := float64(sr.G) / 255
	sB := float64(sr.B) / 255
	
	switch mode {
	case BlendMultiply:
		r = bR * sR
		g = bG * sG
		b = bB * sB
	case BlendScreen:
		r = 1 - (1-bR)*(1-sR)
		g = 1 - (1-bG)*(1-sG)
		b = 1 - (1-bB)*(1-sB)
	case BlendOverlay:
		r = blendOverlay(bR, sR)
		g = blendOverlay(bG, sG)
		b = blendOverlay(bB, sB)
	case BlendDarken:
		r = math.Min(bR, sR)
		g = math.Min(bG, sG)
		b = math.Min(bB, sB)
	case BlendLighten:
		r = math.Max(bR, sR)
		g = math.Max(bG, sG)
		b = math.Max(bB, sB)
	case BlendDifference:
		r = math.Abs(bR - sR)
		g = math.Abs(bG - sG)
		b = math.Abs(bB - sB)
	default: // Normal
		r = sR
		g = sG
		b = sB
	}
	
	return NewRGB(r, g, b)
}

func blendOverlay(b, s float64) float64 {
	if b < 0.5 {
		return 2 * b * s
	}
	return 1 - 2*(1-b)*(1-s)
}
