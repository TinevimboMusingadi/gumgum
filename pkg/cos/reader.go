package cos

import (
	"compress/zlib"
	"fmt"
	"io"
	"os"
)

// Reader provides high-level access to a PDF document's object structure.
type Reader struct {
	data   []byte
	xref   *XrefTable
	cache  map[int]Object // Cache of resolved objects
	objStm map[int]map[int]Object // Cache of objects from object streams
}

// Open opens a PDF file and creates a Reader.
func Open(path string) (*Reader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}
	return NewReader(data)
}

// NewReader creates a Reader from PDF data.
func NewReader(data []byte) (*Reader, error) {
	r := &Reader{
		data:   data,
		cache:  make(map[int]Object),
		objStm: make(map[int]map[int]Object),
	}

	// Find startxref
	startXref, err := findStartXref(data)
	if err != nil {
		return nil, fmt.Errorf("failed to find startxref: %w", err)
	}

	// Parse xref table
	r.xref, err = ParseXref(data, startXref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref: %w", err)
	}

	// Handle prev xref (for incremental updates)
	if prevOffset, ok := r.xref.Trailer.GetInt("Prev"); ok {
		if err := r.loadPrevXref(prevOffset); err != nil {
			// Non-fatal, continue with what we have
		}
	}

	return r, nil
}

// loadPrevXref loads previous xref tables for incremental updates.
func (r *Reader) loadPrevXref(offset int64) error {
	prevXref, err := ParseXref(r.data, offset)
	if err != nil {
		return err
	}

	// Merge entries (current takes precedence)
	for objNum, entry := range prevXref.Entries {
		if _, exists := r.xref.Entries[objNum]; !exists {
			r.xref.Entries[objNum] = entry
		}
	}

	// Recurse for older xrefs
	if prevPrev, ok := prevXref.Trailer.GetInt("Prev"); ok {
		return r.loadPrevXref(prevPrev)
	}

	return nil
}

// Trailer returns the document trailer dictionary.
func (r *Reader) Trailer() Dict {
	return r.xref.Trailer
}

// GetObject retrieves an object by its number, resolving references.
func (r *Reader) GetObject(objNum int) (Object, error) {
	// Check cache
	if obj, ok := r.cache[objNum]; ok {
		return obj, nil
	}

	entry, ok := r.xref.Entries[objNum]
	if !ok {
		return nil, fmt.Errorf("object %d not found in xref", objNum)
	}

	if !entry.InUse {
		return Null{}, nil
	}

	var obj Object
	var err error

	if entry.ObjectStreamNum > 0 {
		// Object is in an object stream
		obj, err = r.getObjectFromStream(entry.ObjectStreamNum, entry.IndexInStream, objNum)
	} else {
		// Object is at file offset
		obj, err = r.getObjectAtOffset(entry.Offset, objNum)
	}

	if err != nil {
		return nil, err
	}

	// Cache the result
	r.cache[objNum] = obj
	return obj, nil
}

// getObjectAtOffset reads an indirect object at the given offset.
func (r *Reader) getObjectAtOffset(offset int64, expectedObjNum int) (Object, error) {
	indirect, err := ParseObjectAt(r.data, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to parse object at offset %d: %w", offset, err)
	}

	// Handle streams that need decompression for Length reference
	if stream, ok := indirect.Object.(*Stream); ok {
		if ref, ok := stream.Dict.Get("Length").(*Reference); ok {
			lengthObj, err := r.GetObject(ref.ObjectNumber)
			if err == nil {
				if length, ok := lengthObj.(Integer); ok {
					// Re-read with correct length
					stream.Dict[Name("Length")] = length
				}
			}
		}
	}

	return indirect.Object, nil
}

// getObjectFromStream retrieves an object from an object stream.
func (r *Reader) getObjectFromStream(streamObjNum, index, targetObjNum int) (Object, error) {
	// Check if we've already parsed this object stream
	if objects, ok := r.objStm[streamObjNum]; ok {
		if obj, ok := objects[targetObjNum]; ok {
			return obj, nil
		}
	}

	// Get the object stream
	streamObj, err := r.GetObject(streamObjNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get object stream %d: %w", streamObjNum, err)
	}

	stream, ok := streamObj.(*Stream)
	if !ok {
		return nil, fmt.Errorf("object %d is not a stream", streamObjNum)
	}

	// Decode the stream
	decoded, err := r.DecodeStream(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to decode object stream: %w", err)
	}

	// Parse objects from stream
	objects, err := ParseObjectsFromStream(decoded, stream.Dict)
	if err != nil {
		return nil, fmt.Errorf("failed to parse object stream contents: %w", err)
	}

	// Cache the parsed objects
	r.objStm[streamObjNum] = objects

	if obj, ok := objects[targetObjNum]; ok {
		return obj, nil
	}

	return nil, fmt.Errorf("object %d not found in object stream %d", targetObjNum, streamObjNum)
}

// Resolve resolves a reference to its actual object.
func (r *Reader) Resolve(obj Object) (Object, error) {
	ref, ok := obj.(*Reference)
	if !ok {
		return obj, nil
	}
	return r.GetObject(ref.ObjectNumber)
}

// ResolveDict resolves a reference and asserts it's a dictionary.
func (r *Reader) ResolveDict(obj Object) (Dict, error) {
	resolved, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	if dict, ok := resolved.(Dict); ok {
		return dict, nil
	}
	return nil, fmt.Errorf("expected Dict, got %T", resolved)
}

// ResolveArray resolves a reference and asserts it's an array.
func (r *Reader) ResolveArray(obj Object) (Array, error) {
	resolved, err := r.Resolve(obj)
	if err != nil {
		return nil, err
	}
	if arr, ok := resolved.(Array); ok {
		return arr, nil
	}
	return nil, fmt.Errorf("expected Array, got %T", resolved)
}

// DecodeStream decodes a stream's data based on its Filter.
func (r *Reader) DecodeStream(s *Stream) ([]byte, error) {
	data := s.Data

	// Get filter(s)
	filter := s.Dict.Get("Filter")
	if filter == nil {
		return data, nil // No filter, return raw data
	}

	// Resolve if reference
	filter, _ = r.Resolve(filter)

	// Handle single filter or array of filters
	var filters []Name
	switch f := filter.(type) {
	case Name:
		filters = []Name{f}
	case Array:
		for _, item := range f {
			resolved, _ := r.Resolve(item)
			if n, ok := resolved.(Name); ok {
				filters = append(filters, n)
			}
		}
	}

	// Apply each filter
	for _, f := range filters {
		var err error
		switch f {
		case "FlateDecode":
			data, err = decodeFlateDecode(data, s.Dict)
		case "ASCIIHexDecode":
			data, err = decodeASCIIHex(data)
		case "ASCII85Decode":
			data, err = decodeASCII85(data)
		case "LZWDecode":
			data, err = decodeLZW(data, s.Dict)
		default:
			// Unknown filter, return what we have
			return data, fmt.Errorf("unsupported filter: %s", f)
		}
		if err != nil {
			return nil, fmt.Errorf("filter %s failed: %w", f, err)
		}
	}

	return data, nil
}

// decodeFlateDecode applies zlib decompression.
func decodeFlateDecode(data []byte, dict Dict) ([]byte, error) {
	r, err := zlib.NewReader(io.NopCloser(
		&byteReader{data: data},
	))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	decoded, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// Apply predictor if present
	if params, ok := dict.GetDict("DecodeParms"); ok {
		decoded, err = applyPredictor(decoded, params)
		if err != nil {
			return nil, err
		}
	}

	return decoded, nil
}

// byteReader wraps a byte slice for io.Reader interface.
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// applyPredictor applies PNG predictor to decoded data.
func applyPredictor(data []byte, params Dict) ([]byte, error) {
	predictor, _ := params.GetInt("Predictor")
	if predictor <= 1 {
		return data, nil // No predictor or TIFF predictor 1
	}

	columns, _ := params.GetInt("Columns")
	if columns == 0 {
		columns = 1
	}

	colors, _ := params.GetInt("Colors")
	if colors == 0 {
		colors = 1
	}

	bpc, _ := params.GetInt("BitsPerComponent")
	if bpc == 0 {
		bpc = 8
	}

	if predictor >= 10 {
		// PNG predictor
		return applyPNGPredictor(data, int(columns), int(colors), int(bpc))
	}

	return data, nil
}

// applyPNGPredictor decodes PNG-filtered data.
func applyPNGPredictor(data []byte, columns, colors, bpc int) ([]byte, error) {
	bytesPerPixel := (colors * bpc + 7) / 8
	rowSize := (columns*colors*bpc + 7) / 8
	inputRowSize := rowSize + 1 // +1 for filter byte

	if len(data) == 0 {
		return data, nil
	}

	numRows := len(data) / inputRowSize
	if numRows == 0 {
		return data, nil
	}

	result := make([]byte, 0, numRows*rowSize)
	prevRow := make([]byte, rowSize)

	for row := 0; row < numRows; row++ {
		start := row * inputRowSize
		if start >= len(data) {
			break
		}

		filterType := data[start]
		rowData := data[start+1:]
		if len(rowData) > rowSize {
			rowData = rowData[:rowSize]
		}

		decoded := make([]byte, rowSize)
		copy(decoded, rowData)

		switch filterType {
		case 0: // None
			// No change needed
		case 1: // Sub
			for i := bytesPerPixel; i < len(decoded); i++ {
				decoded[i] += decoded[i-bytesPerPixel]
			}
		case 2: // Up
			for i := 0; i < len(decoded); i++ {
				decoded[i] += prevRow[i]
			}
		case 3: // Average
			for i := 0; i < len(decoded); i++ {
				var left, up byte
				if i >= bytesPerPixel {
					left = decoded[i-bytesPerPixel]
				}
				up = prevRow[i]
				decoded[i] += byte((int(left) + int(up)) / 2)
			}
		case 4: // Paeth
			for i := 0; i < len(decoded); i++ {
				var a, b, c byte
				if i >= bytesPerPixel {
					a = decoded[i-bytesPerPixel]
					c = prevRow[i-bytesPerPixel]
				}
				b = prevRow[i]
				decoded[i] += paeth(a, b, c)
			}
		}

		result = append(result, decoded...)
		copy(prevRow, decoded)
	}

	return result, nil
}

// paeth implements the Paeth predictor function.
func paeth(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa := abs(p - int(a))
	pb := abs(p - int(b))
	pc := abs(p - int(c))
	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// decodeASCIIHex decodes ASCII hex encoded data.
func decodeASCIIHex(data []byte) ([]byte, error) {
	var result []byte
	var hex byte
	var hasNibble bool

	for _, b := range data {
		if b == '>' {
			break
		}
		if isWhitespace(b) {
			continue
		}

		var nibble byte
		if b >= '0' && b <= '9' {
			nibble = b - '0'
		} else if b >= 'A' && b <= 'F' {
			nibble = b - 'A' + 10
		} else if b >= 'a' && b <= 'f' {
			nibble = b - 'a' + 10
		} else {
			continue
		}

		if hasNibble {
			result = append(result, hex<<4|nibble)
			hasNibble = false
		} else {
			hex = nibble
			hasNibble = true
		}
	}

	if hasNibble {
		result = append(result, hex<<4)
	}

	return result, nil
}

// decodeASCII85 decodes ASCII85 (Base85) encoded data.
func decodeASCII85(data []byte) ([]byte, error) {
	var result []byte
	var tuple uint32
	var count int

	for _, b := range data {
		if b == '~' {
			break // End marker ~>
		}
		if isWhitespace(b) {
			continue
		}
		if b == 'z' && count == 0 {
			result = append(result, 0, 0, 0, 0)
			continue
		}

		if b < '!' || b > 'u' {
			continue
		}

		tuple = tuple*85 + uint32(b-'!')
		count++

		if count == 5 {
			result = append(result,
				byte(tuple>>24),
				byte(tuple>>16),
				byte(tuple>>8),
				byte(tuple),
			)
			tuple = 0
			count = 0
		}
	}

	// Handle remaining bytes
	if count > 0 {
		for i := count; i < 5; i++ {
			tuple = tuple*85 + 84
		}
		for i := 0; i < count-1; i++ {
			result = append(result, byte(tuple>>(24-i*8)))
		}
	}

	return result, nil
}

// decodeLZW decodes LZW compressed data.
func decodeLZW(data []byte, dict Dict) ([]byte, error) {
	// Basic LZW decoder - this is a simplified implementation
	// Full implementation would handle all edge cases
	
	// For now, return an error as LZW is less common
	return nil, fmt.Errorf("LZW decoding not fully implemented")
}

// Catalog returns the document catalog dictionary.
func (r *Reader) Catalog() (Dict, error) {
	rootRef, ok := r.xref.Trailer.GetRef("Root")
	if !ok {
		return nil, fmt.Errorf("no Root in trailer")
	}
	return r.ResolveDict(rootRef)
}

// Pages returns the root pages dictionary.
func (r *Reader) Pages() (Dict, error) {
	catalog, err := r.Catalog()
	if err != nil {
		return nil, err
	}
	
	pagesRef := catalog.Get("Pages")
	if pagesRef == nil {
		return nil, fmt.Errorf("no Pages in catalog")
	}
	
	return r.ResolveDict(pagesRef)
}

// PageCount returns the total number of pages.
func (r *Reader) PageCount() (int, error) {
	pages, err := r.Pages()
	if err != nil {
		return 0, err
	}
	
	count, ok := pages.GetInt("Count")
	if !ok {
		return 0, fmt.Errorf("no Count in Pages")
	}
	
	return int(count), nil
}

// GetPage returns the dictionary for a specific page (0-indexed).
func (r *Reader) GetPage(pageNum int) (Dict, error) {
	pages, err := r.Pages()
	if err != nil {
		return nil, err
	}
	
	return r.findPage(pages, pageNum, 0)
}

// findPage recursively searches the page tree for the given page number.
func (r *Reader) findPage(node Dict, targetPage, currentPage int) (Dict, error) {
	nodeType, _ := node.GetName("Type")
	
	if nodeType == "Page" {
		if currentPage == targetPage {
			return node, nil
		}
		return nil, fmt.Errorf("page not found")
	}
	
	// It's a Pages node
	kids := node.Get("Kids")
	if kids == nil {
		return nil, fmt.Errorf("Pages node without Kids")
	}
	
	kidsArray, err := r.ResolveArray(kids)
	if err != nil {
		return nil, err
	}
	
	pageIndex := currentPage
	for _, kid := range kidsArray {
		kidDict, err := r.ResolveDict(kid)
		if err != nil {
			continue
		}
		
		kidType, _ := kidDict.GetName("Type")
		
		if kidType == "Page" {
			if pageIndex == targetPage {
				return kidDict, nil
			}
			pageIndex++
		} else {
			// Pages node
			count, _ := kidDict.GetInt("Count")
			if pageIndex+int(count) > targetPage {
				return r.findPage(kidDict, targetPage, pageIndex)
			}
			pageIndex += int(count)
		}
	}
	
	return nil, fmt.Errorf("page %d not found", targetPage)
}

// GetPageContents returns the decoded content stream(s) for a page.
func (r *Reader) GetPageContents(page Dict) ([]byte, error) {
	contents := page.Get("Contents")
	if contents == nil {
		return nil, nil // Page with no content
	}
	
	// Resolve if reference
	resolved, err := r.Resolve(contents)
	if err != nil {
		return nil, err
	}
	
	switch c := resolved.(type) {
	case *Stream:
		return r.DecodeStream(c)
	case Array:
		// Multiple content streams - concatenate
		var result []byte
		for _, item := range c {
			streamObj, err := r.Resolve(item)
			if err != nil {
				continue
			}
			if stream, ok := streamObj.(*Stream); ok {
				decoded, err := r.DecodeStream(stream)
				if err != nil {
					continue
				}
				result = append(result, decoded...)
				result = append(result, '\n')
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("unexpected Contents type: %T", c)
	}
}

// Info returns the document info dictionary if present.
func (r *Reader) Info() (Dict, error) {
	infoRef := r.xref.Trailer.Get("Info")
	if infoRef == nil {
		return nil, nil
	}
	return r.ResolveDict(infoRef)
}
