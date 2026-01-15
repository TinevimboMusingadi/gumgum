package main

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gumgum/internal/gui"
	"gumgum/pkg/api"
	"gumgum/pkg/graphics"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "info":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gumgum info <file.pdf>")
			os.Exit(1)
		}
		cmdInfo(os.Args[2])

	case "stream":
		if len(os.Args) < 4 {
			fmt.Println("Usage: gumgum stream <file.pdf> <page>")
			os.Exit(1)
		}
		page, _ := strconv.Atoi(os.Args[3])
		cmdStream(os.Args[2], page)

	case "ops":
		if len(os.Args) < 4 {
			fmt.Println("Usage: gumgum ops <file.pdf> <page>")
			os.Exit(1)
		}
		page, _ := strconv.Atoi(os.Args[3])
		cmdOps(os.Args[2], page)

	case "render":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gumgum render <file.pdf> [-o output.png] [-p page] [-dpi value]")
			os.Exit(1)
		}
		cmdRender(os.Args[2:])

	case "gui":
		if len(os.Args) < 3 {
			cmdGUI(nil)
		} else {
			cmdGUI(os.Args[2:])
		}

	case "help", "-h", "--help":
		printUsage()

	default:
		// If it looks like a PDF file, open GUI
		if strings.HasSuffix(strings.ToLower(os.Args[1]), ".pdf") {
			cmdGUI(os.Args[1:])
		} else {
			fmt.Printf("Unknown command: %s\n", command)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println(`
   ██████╗ ██╗   ██╗███╗   ███╗ ██████╗ ██╗   ██╗███╗   ███╗
  ██╔════╝ ██║   ██║████╗ ████║██╔════╝ ██║   ██║████╗ ████║
  ██║  ███╗██║   ██║██╔████╔██║██║  ███╗██║   ██║██╔████╔██║
  ██║   ██║██║   ██║██║╚██╔╝██║██║   ██║██║   ██║██║╚██╔╝██║
  ╚██████╔╝╚██████╔╝██║ ╚═╝ ██║╚██████╔╝╚██████╔╝██║ ╚═╝ ██║
   ╚═════╝  ╚═════╝ ╚═╝     ╚═╝ ╚═════╝  ╚═════╝ ╚═╝     ╚═╝
                                             
  A custom PDF renderer written in Go from scratch

Usage:
  gumgum <command> [arguments]

Commands:
  info <file.pdf>              Show PDF metadata and page count
  stream <file.pdf> <page>     Dump raw content stream for a page
  ops <file.pdf> <page>        List drawing operations for a page
  render <file.pdf> [options]  Render a page to PNG
    -o <output.png>            Output file (default: output.png)
    -p <page>                  Page number, 0-indexed (default: 0)
    -dpi <value>               Resolution (default: 150)
  gui [file.pdf]               Open GUI viewer
  <file.pdf>                   Open PDF in GUI viewer (shortcut)

Examples:
  gumgum info document.pdf
  gumgum stream document.pdf 0
  gumgum render document.pdf -o page1.png -p 0 -dpi 300
  gumgum document.pdf

Built with:
  - Custom PDF COS parser
  - Custom TrueType font parser
  - golang.org/x/image/vector for rasterization
  - fyne.io for native GUI`)
}

func cmdInfo(path string) {
	doc, err := api.Open(path)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}
	defer doc.Close()

	fmt.Printf("File: %s\n", path)
	fmt.Println("────────────────────────────────────────")
	fmt.Printf("Pages: %d\n", doc.PageCount())

	// Document info
	info := doc.Info()
	if info.Title != "" {
		fmt.Printf("Title: %s\n", info.Title)
	}
	if info.Author != "" {
		fmt.Printf("Author: %s\n", info.Author)
	}
	if info.Subject != "" {
		fmt.Printf("Subject: %s\n", info.Subject)
	}
	if info.Creator != "" {
		fmt.Printf("Creator: %s\n", info.Creator)
	}
	if info.Producer != "" {
		fmt.Printf("Producer: %s\n", info.Producer)
	}
	if info.CreationDate != "" {
		fmt.Printf("Created: %s\n", info.CreationDate)
	}
	if info.ModDate != "" {
		fmt.Printf("Modified: %s\n", info.ModDate)
	}

	// First page info
	if doc.PageCount() > 0 {
		page, err := doc.Page(0)
		if err == nil {
			size := page.Size()
			fmt.Println("\nFirst Page:")
			fmt.Printf("  Size: %.2f × %.2f points (%.2f × %.2f inches)\n",
				size.Width, size.Height,
				size.Width/72, size.Height/72)
			if page.Rotation() != 0 {
				fmt.Printf("  Rotation: %d°\n", page.Rotation())
			}
		}
	}
}

func cmdStream(path string, pageNum int) {
	doc, err := api.Open(path)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}
	defer doc.Close()

	if pageNum < 0 || pageNum >= doc.PageCount() {
		fmt.Printf("Page %d out of range (0-%d)\n", pageNum, doc.PageCount()-1)
		os.Exit(1)
	}

	page, err := doc.Page(pageNum)
	if err != nil {
		fmt.Printf("Error getting page: %v\n", err)
		os.Exit(1)
	}

	contents, err := page.Contents()
	if err != nil {
		fmt.Printf("Error getting page contents: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Page %d Content Stream (%d bytes) ===\n\n", pageNum, len(contents))
	fmt.Println(string(contents))
}

func cmdOps(path string, pageNum int) {
	doc, err := api.Open(path)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}
	defer doc.Close()

	if pageNum < 0 || pageNum >= doc.PageCount() {
		fmt.Printf("Page %d out of range (0-%d)\n", pageNum, doc.PageCount()-1)
		os.Exit(1)
	}

	page, err := doc.Page(pageNum)
	if err != nil {
		fmt.Printf("Error getting page: %v\n", err)
		os.Exit(1)
	}

	contents, err := page.Contents()
	if err != nil {
		fmt.Printf("Error getting page contents: %v\n", err)
		os.Exit(1)
	}

	ops, err := graphics.ParseContentStream(contents)
	if err != nil {
		fmt.Printf("Error parsing content stream: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== Page %d Operations (%d total) ===\n\n", pageNum, len(ops))

	for i, op := range ops {
		if len(op.Operands) > 0 {
			fmt.Printf("%4d: %v %s\n", i+1, op.Operands, op.Name)
		} else {
			fmt.Printf("%4d: %s\n", i+1, op.Name)
		}
	}
}

func cmdRender(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: gumgum render <file.pdf> [-o output.png] [-p page] [-dpi value]")
		os.Exit(1)
	}

	path := args[0]
	output := "output.png"
	pageNum := 0
	dpi := 150.0

	// Parse arguments
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "-o":
			if i+1 < len(args) {
				output = args[i+1]
				i++
			}
		case "-p":
			if i+1 < len(args) {
				pageNum, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "-dpi":
			if i+1 < len(args) {
				dpi, _ = strconv.ParseFloat(args[i+1], 64)
				i++
			}
		}
	}

	fmt.Printf("Opening %s...\n", path)

	doc, err := api.Open(path)
	if err != nil {
		fmt.Printf("Error opening PDF: %v\n", err)
		os.Exit(1)
	}
	defer doc.Close()

	if pageNum < 0 || pageNum >= doc.PageCount() {
		fmt.Printf("Page %d out of range (0-%d)\n", pageNum, doc.PageCount()-1)
		os.Exit(1)
	}

	fmt.Printf("Rendering page %d at %.0f DPI...\n", pageNum, dpi)

	opts := api.WithDPI(dpi)
	img, err := doc.RenderWithOptions(pageNum, opts)
	if err != nil {
		fmt.Printf("Error rendering page: %v\n", err)
		os.Exit(1)
	}

	// Ensure output directory exists
	dir := filepath.Dir(output)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}

	// Save PNG
	f, err := os.Create(output)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		fmt.Printf("Error encoding PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved %s (%dx%d pixels)\n", output, img.Bounds().Dx(), img.Bounds().Dy())
}

func cmdGUI(args []string) {
	app := gui.NewApp()

	if len(args) > 0 {
		app.RunWithFile(args[0])
	} else {
		app.Run()
	}
}
