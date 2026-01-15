package gui

import (
	"image"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"
)

// PageViewer is a custom widget for viewing PDF pages with pan/zoom.
type PageViewer struct {
	widget.BaseWidget
	
	image     *canvas.Image
	pageImg   image.Image
	
	// View state
	zoom      float64
	offsetX   float64
	offsetY   float64
	
	// Dragging state
	dragging  bool
	dragStart fyne.Position
	startOffsetX float64
	startOffsetY float64
}

// NewPageViewer creates a new page viewer widget.
func NewPageViewer() *PageViewer {
	v := &PageViewer{
		zoom: 1.0,
	}
	v.ExtendBaseWidget(v)
	
	v.image = canvas.NewImageFromImage(nil)
	v.image.FillMode = canvas.ImageFillOriginal
	v.image.ScaleMode = canvas.ImageScaleSmooth
	
	return v
}

// SetImage sets the page image to display.
func (v *PageViewer) SetImage(img image.Image) {
	v.pageImg = img
	v.image.Image = img
	v.resetView()
	v.Refresh()
}

// resetView resets zoom and offset.
func (v *PageViewer) resetView() {
	v.zoom = 1.0
	v.offsetX = 0
	v.offsetY = 0
}

// CreateRenderer creates the renderer for this widget.
func (v *PageViewer) CreateRenderer() fyne.WidgetRenderer {
	return &pageViewerRenderer{
		viewer: v,
	}
}

// Dragged handles drag events for panning.
func (v *PageViewer) Dragged(event *fyne.DragEvent) {
	v.offsetX = v.startOffsetX + float64(event.Dragged.DX)
	v.offsetY = v.startOffsetY + float64(event.Dragged.DY)
	v.Refresh()
}

// DragEnd handles the end of a drag.
func (v *PageViewer) DragEnd() {
	v.startOffsetX = v.offsetX
	v.startOffsetY = v.offsetY
}

// Scrolled handles scroll events for zooming.
func (v *PageViewer) Scrolled(event *fyne.ScrollEvent) {
	delta := float64(event.Scrolled.DY) / 100
	newZoom := v.zoom * (1 + delta)
	
	// Clamp zoom
	newZoom = math.Max(0.1, math.Min(5.0, newZoom))
	
	// Zoom toward cursor position
	if v.pageImg != nil {
		// Get cursor position relative to image center
		size := v.Size()
		imgW := float64(v.pageImg.Bounds().Dx()) * v.zoom
		imgH := float64(v.pageImg.Bounds().Dy()) * v.zoom
		
		centerX := float64(size.Width) / 2
		centerY := float64(size.Height) / 2
		
		cursorX := float64(event.Position.X)
		cursorY := float64(event.Position.Y)
		
		// Adjust offset to zoom toward cursor
		if v.zoom != newZoom {
			factor := newZoom / v.zoom
			v.offsetX = cursorX - (cursorX-centerX-v.offsetX)*factor - (centerX - imgW*factor/2)
			v.offsetY = cursorY - (cursorY-centerY-v.offsetY)*factor - (centerY - imgH*factor/2)
		}
	}
	
	v.zoom = newZoom
	v.Refresh()
}

// ZoomIn increases zoom level.
func (v *PageViewer) ZoomIn() {
	v.zoom = math.Min(5.0, v.zoom*1.2)
	v.Refresh()
}

// ZoomOut decreases zoom level.
func (v *PageViewer) ZoomOut() {
	v.zoom = math.Max(0.1, v.zoom/1.2)
	v.Refresh()
}

// FitWidth fits the image to the widget width.
func (v *PageViewer) FitWidth() {
	if v.pageImg == nil {
		return
	}
	
	size := v.Size()
	imgW := float64(v.pageImg.Bounds().Dx())
	
	v.zoom = float64(size.Width) / imgW
	v.offsetX = 0
	v.offsetY = 0
	v.Refresh()
}

// FitPage fits the entire page in the widget.
func (v *PageViewer) FitPage() {
	if v.pageImg == nil {
		return
	}
	
	size := v.Size()
	imgW := float64(v.pageImg.Bounds().Dx())
	imgH := float64(v.pageImg.Bounds().Dy())
	
	zoomW := float64(size.Width) / imgW
	zoomH := float64(size.Height) / imgH
	
	v.zoom = math.Min(zoomW, zoomH)
	v.offsetX = 0
	v.offsetY = 0
	v.Refresh()
}

// pageViewerRenderer renders the page viewer.
type pageViewerRenderer struct {
	viewer *PageViewer
}

func (r *pageViewerRenderer) Layout(size fyne.Size) {
	if r.viewer.pageImg == nil {
		return
	}
	
	imgW := float32(r.viewer.pageImg.Bounds().Dx()) * float32(r.viewer.zoom)
	imgH := float32(r.viewer.pageImg.Bounds().Dy()) * float32(r.viewer.zoom)
	
	// Center image with offset
	x := (size.Width-imgW)/2 + float32(r.viewer.offsetX)
	y := (size.Height-imgH)/2 + float32(r.viewer.offsetY)
	
	r.viewer.image.Move(fyne.NewPos(x, y))
	r.viewer.image.Resize(fyne.NewSize(imgW, imgH))
}

func (r *pageViewerRenderer) MinSize() fyne.Size {
	return fyne.NewSize(200, 200)
}

func (r *pageViewerRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.viewer.image}
}

func (r *pageViewerRenderer) Refresh() {
	r.viewer.image.Refresh()
}

func (r *pageViewerRenderer) Destroy() {}
