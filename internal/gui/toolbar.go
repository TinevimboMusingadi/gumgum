package gui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Toolbar provides navigation and zoom controls.
type Toolbar struct {
	container *fyne.Container
	
	// Callbacks
	OnOpen     func()
	OnPrev     func()
	OnNext     func()
	OnFirst    func()
	OnLast     func()
	OnGoTo     func(page int)
	OnZoomIn   func()
	OnZoomOut  func()
	OnFitWidth func()
	OnFitPage  func()
	
	// Components
	pageEntry  *widget.Entry
	pageLabel  *widget.Label
	prevBtn    *widget.Button
	nextBtn    *widget.Button
	
	// State
	currentPage int
	totalPages  int
}

// NewToolbar creates a new toolbar.
func NewToolbar() *Toolbar {
	t := &Toolbar{}
	t.build()
	return t
}

func (t *Toolbar) build() {
	// Open button
	openBtn := widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), func() {
		if t.OnOpen != nil {
			t.OnOpen()
		}
	})
	
	// Navigation buttons
	firstBtn := widget.NewButtonWithIcon("", theme.MediaSkipPreviousIcon(), func() {
		if t.OnFirst != nil {
			t.OnFirst()
		}
	})
	
	t.prevBtn = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if t.OnPrev != nil {
			t.OnPrev()
		}
	})
	
	t.nextBtn = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), func() {
		if t.OnNext != nil {
			t.OnNext()
		}
	})
	
	lastBtn := widget.NewButtonWithIcon("", theme.MediaSkipNextIcon(), func() {
		if t.OnLast != nil {
			t.OnLast()
		}
	})
	
	// Page entry
	t.pageEntry = widget.NewEntry()
	t.pageEntry.SetPlaceHolder("Page")
	t.pageEntry.OnSubmitted = func(s string) {
		if page, err := strconv.Atoi(s); err == nil && t.OnGoTo != nil {
			t.OnGoTo(page - 1) // Convert to 0-indexed
		}
	}
	t.pageEntry.Resize(fyne.NewSize(60, t.pageEntry.MinSize().Height))
	
	t.pageLabel = widget.NewLabel("of 0")
	
	// Zoom buttons
	zoomOutBtn := widget.NewButtonWithIcon("", theme.ZoomOutIcon(), func() {
		if t.OnZoomOut != nil {
			t.OnZoomOut()
		}
	})
	
	zoomInBtn := widget.NewButtonWithIcon("", theme.ZoomInIcon(), func() {
		if t.OnZoomIn != nil {
			t.OnZoomIn()
		}
	})
	
	fitWidthBtn := widget.NewButtonWithIcon("Width", theme.ViewFullScreenIcon(), func() {
		if t.OnFitWidth != nil {
			t.OnFitWidth()
		}
	})
	
	fitPageBtn := widget.NewButtonWithIcon("Page", theme.ViewRestoreIcon(), func() {
		if t.OnFitPage != nil {
			t.OnFitPage()
		}
	})
	
	// Build container
	t.container = container.NewHBox(
		openBtn,
		widget.NewSeparator(),
		firstBtn,
		t.prevBtn,
		container.NewHBox(t.pageEntry, t.pageLabel),
		t.nextBtn,
		lastBtn,
		widget.NewSeparator(),
		zoomOutBtn,
		zoomInBtn,
		widget.NewSeparator(),
		fitWidthBtn,
		fitPageBtn,
	)
}

// Container returns the toolbar container.
func (t *Toolbar) Container() *fyne.Container {
	return t.container
}

// SetPage updates the current page display.
func (t *Toolbar) SetPage(current, total int) {
	t.currentPage = current
	t.totalPages = total
	
	t.pageEntry.SetText(strconv.Itoa(current + 1))
	t.pageLabel.SetText("of " + strconv.Itoa(total))
	
	// Update button states
	if current <= 0 {
		t.prevBtn.Disable()
	} else {
		t.prevBtn.Enable()
	}
	
	if current >= total-1 {
		t.nextBtn.Disable()
	} else {
		t.nextBtn.Enable()
	}
}

// Enable enables the toolbar controls.
func (t *Toolbar) Enable() {
	t.prevBtn.Enable()
	t.nextBtn.Enable()
	t.pageEntry.Enable()
}

// Disable disables the toolbar controls.
func (t *Toolbar) Disable() {
	t.prevBtn.Disable()
	t.nextBtn.Disable()
	t.pageEntry.Disable()
	t.pageEntry.SetText("")
	t.pageLabel.SetText("of 0")
}

// StatusBar provides status information.
type StatusBar struct {
	container *fyne.Container
	label     *widget.Label
	zoomLabel *widget.Label
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	s := &StatusBar{
		label:     widget.NewLabel("Ready"),
		zoomLabel: widget.NewLabel("100%"),
	}
	
	s.container = container.NewHBox(
		s.label,
		widget.NewSeparator(),
		s.zoomLabel,
	)
	
	return s
}

// Container returns the status bar container.
func (s *StatusBar) Container() *fyne.Container {
	return s.container
}

// SetStatus sets the status message.
func (s *StatusBar) SetStatus(msg string) {
	s.label.SetText(msg)
}

// SetZoom sets the zoom percentage display.
func (s *StatusBar) SetZoom(percent int) {
	s.zoomLabel.SetText(strconv.Itoa(percent) + "%")
}
