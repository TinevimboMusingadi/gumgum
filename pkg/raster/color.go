package raster

import (
	"image/color"
	"math"

	"gumgum/pkg/graphics"
)

// ColorConverter handles color space conversions for rendering.
type ColorConverter struct {
	// ICC profiles would go here for accurate color management
}

// NewColorConverter creates a new color converter.
func NewColorConverter() *ColorConverter {
	return &ColorConverter{}
}

// ToRGBA converts a graphics.Color to a standard RGBA color.
func (cc *ColorConverter) ToRGBA(c graphics.Color) color.RGBA {
	return c.ToRGBA()
}

// ToNRGBA converts a graphics.Color to a non-premultiplied RGBA color with alpha.
func (cc *ColorConverter) ToNRGBA(c graphics.Color, alpha float64) color.NRGBA {
	rgba := c.ToRGBA()
	return color.NRGBA{
		R: rgba.R,
		G: rgba.G,
		B: rgba.B,
		A: uint8(clamp(alpha, 0, 1) * 255),
	}
}

// GrayToRGB converts a grayscale value to RGB.
func GrayToRGB(gray float64) (r, g, b float64) {
	gray = clamp(gray, 0, 1)
	return gray, gray, gray
}

// RGBToGray converts RGB to grayscale using luminance.
func RGBToGray(r, g, b float64) float64 {
	// ITU-R BT.709 luma coefficients
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// CMYKToRGB converts CMYK to RGB.
func CMYKToRGB(c, m, y, k float64) (r, g, b float64) {
	c = clamp(c, 0, 1)
	m = clamp(m, 0, 1)
	y = clamp(y, 0, 1)
	k = clamp(k, 0, 1)

	r = (1 - c) * (1 - k)
	g = (1 - m) * (1 - k)
	b = (1 - y) * (1 - k)
	return
}

// RGBToCMYK converts RGB to CMYK.
func RGBToCMYK(r, g, b float64) (c, m, y, k float64) {
	r = clamp(r, 0, 1)
	g = clamp(g, 0, 1)
	b = clamp(b, 0, 1)

	k = 1 - math.Max(r, math.Max(g, b))
	if k == 1 {
		return 0, 0, 0, 1
	}

	c = (1 - r - k) / (1 - k)
	m = (1 - g - k) / (1 - k)
	y = (1 - b - k) / (1 - k)
	return
}

// LabToRGB converts CIE Lab to RGB (D65 illuminant).
func LabToRGB(l, a, b float64) (r, g, bb float64) {
	// Lab to XYZ
	y := (l + 16) / 116
	x := a/500 + y
	z := y - b/200

	// Apply inverse f function
	x3 := x * x * x
	y3 := y * y * y
	z3 := z * z * z

	if x3 > 0.008856 {
		x = x3
	} else {
		x = (x - 16.0/116) / 7.787
	}
	if y3 > 0.008856 {
		y = y3
	} else {
		y = (y - 16.0/116) / 7.787
	}
	if z3 > 0.008856 {
		z = z3
	} else {
		z = (z - 16.0/116) / 7.787
	}

	// D65 reference white
	x *= 0.95047
	y *= 1.0
	z *= 1.08883

	// XYZ to sRGB
	r = x*3.2406 + y*-1.5372 + z*-0.4986
	g = x*-0.9689 + y*1.8758 + z*0.0415
	bb = x*0.0557 + y*-0.2040 + z*1.0570

	// Gamma correction
	r = gammaCorrect(r)
	g = gammaCorrect(g)
	bb = gammaCorrect(bb)

	return clamp(r, 0, 1), clamp(g, 0, 1), clamp(bb, 0, 1)
}

func gammaCorrect(v float64) float64 {
	if v > 0.0031308 {
		return 1.055*math.Pow(v, 1/2.4) - 0.055
	}
	return 12.92 * v
}

// HSVToRGB converts HSV to RGB.
func HSVToRGB(h, s, v float64) (r, g, b float64) {
	if s == 0 {
		return v, v, v
	}

	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	h /= 60
	i := math.Floor(h)
	f := h - i
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	switch int(i) {
	case 0:
		return v, t, p
	case 1:
		return q, v, p
	case 2:
		return p, v, t
	case 3:
		return p, q, v
	case 4:
		return t, p, v
	default:
		return v, p, q
	}
}

// RGBToHSV converts RGB to HSV.
func RGBToHSV(r, g, b float64) (h, s, v float64) {
	max := math.Max(r, math.Max(g, b))
	min := math.Min(r, math.Min(g, b))
	v = max

	d := max - min
	if max == 0 {
		s = 0
	} else {
		s = d / max
	}

	if max == min {
		h = 0
	} else {
		switch max {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		case b:
			h = (r-g)/d + 4
		}
		h *= 60
	}

	return
}

// Blend blends two colors using the specified alpha.
func Blend(dst, src color.Color, alpha float64) color.RGBA {
	dr, dg, db, da := dst.RGBA()
	sr, sg, sb, sa := src.RGBA()

	a := uint32(alpha * 65535)
	invA := 65535 - a

	return color.RGBA{
		R: uint8((sr*a + dr*invA) / 65535 >> 8),
		G: uint8((sg*a + dg*invA) / 65535 >> 8),
		B: uint8((sb*a + db*invA) / 65535 >> 8),
		A: uint8((sa*a + da*invA) / 65535 >> 8),
	}
}

// AlphaBlend performs proper alpha compositing.
func AlphaBlend(dst, src color.NRGBA) color.NRGBA {
	if src.A == 0 {
		return dst
	}
	if src.A == 255 {
		return src
	}

	srcA := float64(src.A) / 255
	dstA := float64(dst.A) / 255
	outA := srcA + dstA*(1-srcA)

	if outA == 0 {
		return color.NRGBA{}
	}

	return color.NRGBA{
		R: uint8((float64(src.R)*srcA + float64(dst.R)*dstA*(1-srcA)) / outA),
		G: uint8((float64(src.G)*srcA + float64(dst.G)*dstA*(1-srcA)) / outA),
		B: uint8((float64(src.B)*srcA + float64(dst.B)*dstA*(1-srcA)) / outA),
		A: uint8(outA * 255),
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

// Palette represents an indexed color palette.
type Palette struct {
	Colors []color.RGBA
}

// NewPalette creates a new palette with the given number of colors.
func NewPalette(size int) *Palette {
	return &Palette{
		Colors: make([]color.RGBA, size),
	}
}

// Get returns the color at the given index.
func (p *Palette) Get(index int) color.RGBA {
	if index >= 0 && index < len(p.Colors) {
		return p.Colors[index]
	}
	return color.RGBA{0, 0, 0, 255}
}

// Set sets the color at the given index.
func (p *Palette) Set(index int, c color.RGBA) {
	if index >= 0 && index < len(p.Colors) {
		p.Colors[index] = c
	}
}

// Nearest finds the nearest color in the palette.
func (p *Palette) Nearest(c color.Color) int {
	if len(p.Colors) == 0 {
		return 0
	}

	r, g, b, _ := c.RGBA()
	r, g, b = r>>8, g>>8, b>>8

	bestIdx := 0
	bestDist := int64(1<<63 - 1)

	for i, pc := range p.Colors {
		dr := int64(r) - int64(pc.R)
		dg := int64(g) - int64(pc.G)
		db := int64(b) - int64(pc.B)
		dist := dr*dr + dg*dg + db*db

		if dist < bestDist {
			bestDist = dist
			bestIdx = i
		}
	}

	return bestIdx
}
