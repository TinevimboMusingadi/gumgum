// Package gui provides a native desktop PDF viewer using Fyne.
package gui

import (
	"fmt"
	"image"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"gumgum/pkg/api"
)

// App represents the PDF viewer application.
type App struct {
	fyneApp    fyne.App
	mainWindow fyne.Window
	document   *api.Document
	currentPage int
	dpi        float64

	// UI components
	pageImage   *canvas.Image
	pageLabel   *widget.Label
	prevButton  *widget.Button
	nextButton  *widget.Button
	zoomInBtn   *widget.Button
	zoomOutBtn  *widget.Button
	scrollContainer *container.Scroll
}

// NewApp creates a new PDF viewer application.
func NewApp() *App {
	a := &App{
		fyneApp: app.New(),
		currentPage: 0,
		dpi: 150,
	}
	
	a.fyneApp.Settings().SetTheme(theme.DarkTheme())
	a.mainWindow = a.fyneApp.NewWindow("GumGum PDF Viewer")
	a.mainWindow.Resize(fyne.NewSize(900, 700))
	
	return a
}

// Run starts the application.
func (a *App) Run() {
	a.buildUI()
	a.mainWindow.ShowAndRun()
}

// RunWithFile starts the application with a file already loaded.
func (a *App) RunWithFile(path string) {
	a.buildUI()
	
	// Load file after window is ready
	go func() {
		if err := a.loadFile(path); err != nil {
			dialog.ShowError(err, a.mainWindow)
		}
	}()
	
	a.mainWindow.ShowAndRun()
}

// buildUI constructs the user interface.
func (a *App) buildUI() {
	// Create placeholder image
	a.pageImage = canvas.NewImageFromImage(image.NewRGBA(image.Rect(0, 0, 1, 1)))
	a.pageImage.FillMode = canvas.ImageFillContain
	a.pageImage.ScaleMode = canvas.ImageScaleSmooth
	
	// Page label
	a.pageLabel = widget.NewLabel("No document loaded")
	a.pageLabel.Alignment = fyne.TextAlignCenter
	
	// Navigation buttons
	a.prevButton = widget.NewButtonWithIcon("", theme.NavigateBackIcon(), a.prevPage)
	a.prevButton.Disable()
	
	a.nextButton = widget.NewButtonWithIcon("", theme.NavigateNextIcon(), a.nextPage)
	a.nextButton.Disable()
	
	// Zoom buttons
	a.zoomInBtn = widget.NewButtonWithIcon("", theme.ZoomInIcon(), a.zoomIn)
	a.zoomOutBtn = widget.NewButtonWithIcon("", theme.ZoomOutIcon(), a.zoomOut)
	
	// Open button
	openBtn := widget.NewButtonWithIcon("Open", theme.FolderOpenIcon(), a.openFile)
	
	// Toolbar
	toolbar := container.NewHBox(
		openBtn,
		widget.NewSeparator(),
		a.prevButton,
		a.pageLabel,
		a.nextButton,
		widget.NewSeparator(),
		a.zoomOutBtn,
		widget.NewLabel("Zoom"),
		a.zoomInBtn,
	)
	
	// Scroll container for the page
	a.scrollContainer = container.NewScroll(a.pageImage)
	
	// Main layout
	content := container.NewBorder(
		container.NewPadded(toolbar), // Top
		nil, // Bottom
		nil, // Left
		nil, // Right
		a.scrollContainer, // Center
	)
	
	a.mainWindow.SetContent(content)
	
	// Set up keyboard shortcuts
	a.mainWindow.Canvas().SetOnTypedKey(a.handleKey)
}

// handleKey handles keyboard navigation.
func (a *App) handleKey(key *fyne.KeyEvent) {
	switch key.Name {
	case fyne.KeyLeft, fyne.KeyUp, fyne.KeyPageUp:
		a.prevPage()
	case fyne.KeyRight, fyne.KeyDown, fyne.KeyPageDown, fyne.KeySpace:
		a.nextPage()
	case fyne.KeyHome:
		a.goToPage(0)
	case fyne.KeyEnd:
		if a.document != nil {
			a.goToPage(a.document.PageCount() - 1)
		}
	case fyne.KeyPlus, fyne.KeyEqual:
		a.zoomIn()
	case fyne.KeyMinus:
		a.zoomOut()
	}
}

// openFile shows a file dialog and loads the selected PDF.
func (a *App) openFile() {
	dialog.ShowFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.mainWindow)
			return
		}
		if reader == nil {
			return // Cancelled
		}
		defer reader.Close()
		
		path := reader.URI().Path()
		if err := a.loadFile(path); err != nil {
			dialog.ShowError(err, a.mainWindow)
		}
	}, a.mainWindow)
}

// loadFile loads a PDF file.
func (a *App) loadFile(path string) error {
	doc, err := api.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open PDF: %w", err)
	}
	
	// Close previous document
	if a.document != nil {
		a.document.Close()
	}
	
	a.document = doc
	a.currentPage = 0
	
	// Update window title
	a.mainWindow.SetTitle(fmt.Sprintf("GumGum - %s", path))
	
	// Enable navigation
	a.updateNavigation()
	
	// Render first page
	return a.renderCurrentPage()
}

// renderCurrentPage renders and displays the current page.
func (a *App) renderCurrentPage() error {
	if a.document == nil {
		return nil
	}
	
	opts := api.WithDPI(a.dpi)
	img, err := a.document.RenderWithOptions(a.currentPage, opts)
	if err != nil {
		return fmt.Errorf("failed to render page: %w", err)
	}
	
	// Update image
	a.pageImage.Image = img
	a.pageImage.SetMinSize(fyne.NewSize(float32(img.Bounds().Dx()), float32(img.Bounds().Dy())))
	a.pageImage.Refresh()
	
	// Reset scroll position
	a.scrollContainer.ScrollToTop()
	
	return nil
}

// updateNavigation updates navigation buttons and label.
func (a *App) updateNavigation() {
	if a.document == nil {
		a.pageLabel.SetText("No document loaded")
		a.prevButton.Disable()
		a.nextButton.Disable()
		return
	}
	
	pageCount := a.document.PageCount()
	a.pageLabel.SetText(fmt.Sprintf("Page %d of %d", a.currentPage+1, pageCount))
	
	if a.currentPage > 0 {
		a.prevButton.Enable()
	} else {
		a.prevButton.Disable()
	}
	
	if a.currentPage < pageCount-1 {
		a.nextButton.Enable()
	} else {
		a.nextButton.Disable()
	}
}

// prevPage navigates to the previous page.
func (a *App) prevPage() {
	if a.document == nil || a.currentPage <= 0 {
		return
	}
	a.currentPage--
	a.updateNavigation()
	a.renderCurrentPage()
}

// nextPage navigates to the next page.
func (a *App) nextPage() {
	if a.document == nil || a.currentPage >= a.document.PageCount()-1 {
		return
	}
	a.currentPage++
	a.updateNavigation()
	a.renderCurrentPage()
}

// goToPage navigates to a specific page.
func (a *App) goToPage(page int) {
	if a.document == nil {
		return
	}
	if page < 0 {
		page = 0
	}
	if page >= a.document.PageCount() {
		page = a.document.PageCount() - 1
	}
	if page != a.currentPage {
		a.currentPage = page
		a.updateNavigation()
		a.renderCurrentPage()
	}
}

// zoomIn increases the DPI.
func (a *App) zoomIn() {
	if a.dpi < 400 {
		a.dpi += 25
		a.renderCurrentPage()
	}
}

// zoomOut decreases the DPI.
func (a *App) zoomOut() {
	if a.dpi > 50 {
		a.dpi -= 25
		a.renderCurrentPage()
	}
}
