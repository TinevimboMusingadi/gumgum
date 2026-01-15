package cos

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// XrefEntry represents a single entry in the cross-reference table.
type XrefEntry struct {
	Offset     int64 // Byte offset in file (for 'n' entries)
	Generation int   // Generation number
	InUse      bool  // true for 'n', false for 'f'
	// For compressed objects (xref streams)
	ObjectStreamNum int // Object number of the stream containing this object
	IndexInStream   int // Index within the object stream
}

// XrefTable maps object numbers to their locations in the file.
type XrefTable struct {
	Entries map[int]*XrefEntry
	Trailer Dict
}

// NewXrefTable creates an empty xref table.
func NewXrefTable() *XrefTable {
	return &XrefTable{
		Entries: make(map[int]*XrefEntry),
	}
}

// findStartXref locates the startxref offset at the end of the PDF.
func findStartXref(data []byte) (int64, error) {
	// Look in the last 1024 bytes
	searchSize := 1024
	if len(data) < searchSize {
		searchSize = len(data)
	}
	tail := data[len(data)-searchSize:]

	// Find "startxref"
	idx := bytes.LastIndex(tail, []byte("startxref"))
	if idx == -1 {
		return 0, fmt.Errorf("startxref not found")
	}

	// Parse the offset number
	after := string(tail[idx+9:]) // Skip "startxref"
	after = strings.TrimSpace(after)

	// Find the number (ends at %%EOF or whitespace)
	var numStr string
	for _, c := range after {
		if c >= '0' && c <= '9' {
			numStr += string(c)
		} else if len(numStr) > 0 {
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("startxref offset not found")
	}

	offset, err := strconv.ParseInt(numStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid startxref offset: %s", numStr)
	}

	return offset, nil
}

// parseXrefTable parses a traditional xref table (not a stream).
func parseXrefTable(data []byte, offset int64) (*XrefTable, error) {
	table := NewXrefTable()
	pos := int(offset)

	// Skip whitespace and find "xref"
	for pos < len(data) && isWhitespace(data[pos]) {
		pos++
	}

	if pos+4 > len(data) || string(data[pos:pos+4]) != "xref" {
		// Might be an xref stream instead
		return nil, fmt.Errorf("xref keyword not found at offset %d", offset)
	}
	pos += 4

	// Parse subsections
	for {
		// Skip whitespace
		for pos < len(data) && isWhitespace(data[pos]) {
			pos++
		}

		if pos >= len(data) {
			break
		}

		// Check for "trailer"
		if pos+7 <= len(data) && string(data[pos:pos+7]) == "trailer" {
			pos += 7
			break
		}

		// Parse "start count" line
		var startObj, count int
		n, err := fmt.Sscanf(string(data[pos:]), "%d %d", &startObj, &count)
		if err != nil || n != 2 {
			break
		}

		// Skip to end of line
		for pos < len(data) && data[pos] != '\n' && data[pos] != '\r' {
			pos++
		}
		for pos < len(data) && (data[pos] == '\n' || data[pos] == '\r') {
			pos++
		}

		// Parse entries
		for i := 0; i < count && pos < len(data); i++ {
			// Each entry is exactly 20 bytes: "nnnnnnnnnn ggggg n \n"
			if pos+20 > len(data) {
				break
			}

			line := string(data[pos : pos+20])
			var entryOffset int64
			var gen int
			var flag byte

			// Try to parse the line
			trimmed := strings.TrimSpace(line)
			parts := strings.Fields(trimmed)
			if len(parts) >= 3 {
				entryOffset, _ = strconv.ParseInt(parts[0], 10, 64)
				gen64, _ := strconv.ParseInt(parts[1], 10, 32)
				gen = int(gen64)
				if len(parts[2]) > 0 {
					flag = parts[2][0]
				}

				objNum := startObj + i
				table.Entries[objNum] = &XrefEntry{
					Offset:     entryOffset,
					Generation: gen,
					InUse:      flag == 'n',
				}
			}

			pos += 20
		}
	}

	// Parse trailer dictionary
	for pos < len(data) && isWhitespace(data[pos]) {
		pos++
	}

	lexer := NewLexer(data[pos:])
	parser := &Parser{lexer: lexer}
	if obj, err := parser.ParseObject(); err == nil {
		if dict, ok := obj.(Dict); ok {
			table.Trailer = dict
		}
	}

	return table, nil
}

// ParseXref attempts to parse the xref table or stream at the given offset.
func ParseXref(data []byte, offset int64) (*XrefTable, error) {
	// First try traditional xref table
	table, err := parseXrefTable(data, offset)
	if err == nil {
		return table, nil
	}

	// If that fails, try xref stream (PDF 1.5+)
	return parseXrefStream(data, offset)
}

// parseXrefStream parses an xref stream (PDF 1.5+).
func parseXrefStream(data []byte, offset int64) (*XrefTable, error) {
	// Position at the object
	lexer := NewLexer(data[offset:])
	parser := &Parser{lexer: lexer}

	// Parse the indirect object
	indirect, err := parser.ParseIndirectObject()
	if err != nil {
		return nil, fmt.Errorf("failed to parse xref stream object: %w", err)
	}

	stream, ok := indirect.Object.(*Stream)
	if !ok {
		return nil, fmt.Errorf("expected stream at xref stream offset")
	}

	// Decompress the stream data before parsing
	decodedData, err := decodeStreamData(stream)
	if err != nil {
		return nil, fmt.Errorf("failed to decode xref stream: %w", err)
	}
	stream.Data = decodedData

	return decodeXrefStream(stream)
}

// decodeXrefStream decodes an xref stream into an XrefTable.
func decodeXrefStream(stream *Stream) (*XrefTable, error) {
	table := NewXrefTable()
	table.Trailer = stream.Dict

	// Get W array (field widths)
	wArray, ok := stream.Dict.GetArray("W")
	if !ok || len(wArray) < 3 {
		return nil, fmt.Errorf("missing or invalid W array in xref stream")
	}

	var w [3]int
	for i := 0; i < 3; i++ {
		if n, ok := wArray[i].(Integer); ok {
			w[i] = int(n)
		}
	}

	entrySize := w[0] + w[1] + w[2]
	if entrySize == 0 {
		return nil, fmt.Errorf("invalid W array: entry size is 0")
	}

	// Get Index array (which objects are in this xref)
	var indices []int
	if indexArray, ok := stream.Dict.GetArray("Index"); ok {
		for _, v := range indexArray {
			if n, ok := v.(Integer); ok {
				indices = append(indices, int(n))
			}
		}
	} else {
		// Default: single subsection starting at 0
		if size, ok := stream.Dict.GetInt("Size"); ok {
			indices = []int{0, int(size)}
		}
	}

	// Parse entries
	data := stream.Data
	pos := 0

	for i := 0; i < len(indices); i += 2 {
		if i+1 >= len(indices) {
			break
		}
		startObj := indices[i]
		count := indices[i+1]

		for j := 0; j < count && pos+entrySize <= len(data); j++ {
			objNum := startObj + j

			// Read fields
			var fields [3]int64
			for f := 0; f < 3; f++ {
				for k := 0; k < w[f]; k++ {
					fields[f] = (fields[f] << 8) | int64(data[pos])
					pos++
				}
			}

			// Default type is 1 if w[0] is 0
			entryType := fields[0]
			if w[0] == 0 {
				entryType = 1
			}

			entry := &XrefEntry{}
			switch entryType {
			case 0: // Free object
				entry.InUse = false
				entry.Offset = fields[1]
				entry.Generation = int(fields[2])
			case 1: // Uncompressed object
				entry.InUse = true
				entry.Offset = fields[1]
				entry.Generation = int(fields[2])
		case 2: // Compressed object in object stream
			entry.InUse = true
			entry.ObjectStreamNum = int(fields[1])
			entry.IndexInStream = int(fields[2])
		}

		table.Entries[objNum] = entry
	}
}

	return table, nil
}

// decodeStreamData decompresses stream data based on Filter.
// This is a standalone version for use before Reader is initialized.
func decodeStreamData(s *Stream) ([]byte, error) {
	data := s.Data

	// Get filter
	filter := s.Dict.Get("Filter")
	if filter == nil {
		return data, nil // No filter
	}

	// Handle single filter
	filterName, ok := filter.(Name)
	if !ok {
		// Could be an array of filters
		if arr, ok := filter.(Array); ok && len(arr) > 0 {
			if n, ok := arr[0].(Name); ok {
				filterName = n
			}
		}
	}

	switch filterName {
	case "FlateDecode":
		return decodeFlate(data, s.Dict)
	default:
		return data, nil
	}
}

// decodeFlate applies zlib decompression with optional predictor.
func decodeFlate(data []byte, dict Dict) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer r.Close()

	decoded, err := io.ReadAll(r)
	if err != nil {
		// Some streams may be truncated, return what we got
		if len(decoded) > 0 {
			err = nil
		} else {
			return nil, err
		}
	}

	// Apply predictor if present
	params := dict.Get("DecodeParms")
	if params == nil {
		return decoded, nil
	}

	var paramsDict Dict
	if d, ok := params.(Dict); ok {
		paramsDict = d
	} else {
		return decoded, nil
	}

	predictor, _ := paramsDict.GetInt("Predictor")
	if predictor <= 1 {
		return decoded, nil
	}

	columns, _ := paramsDict.GetInt("Columns")
	if columns == 0 {
		columns = 1
	}

	// PNG predictor (10-15)
	if predictor >= 10 {
		return applyPNGPredictorXref(decoded, int(columns))
	}

	return decoded, nil
}

// applyPNGPredictorXref decodes PNG-filtered data for xref streams.
func applyPNGPredictorXref(data []byte, columns int) ([]byte, error) {
	rowSize := columns
	inputRowSize := rowSize + 1 // +1 for filter byte

	if len(data) == 0 || inputRowSize == 0 {
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
		rowData := make([]byte, rowSize)
		
		srcStart := start + 1
		srcEnd := srcStart + rowSize
		if srcEnd > len(data) {
			srcEnd = len(data)
		}
		copy(rowData, data[srcStart:srcEnd])

		switch filterType {
		case 0: // None
			// No change
		case 1: // Sub
			for i := 1; i < len(rowData); i++ {
				rowData[i] += rowData[i-1]
			}
		case 2: // Up
			for i := 0; i < len(rowData); i++ {
				rowData[i] += prevRow[i]
			}
		}

		result = append(result, rowData...)
		copy(prevRow, rowData)
	}

	return result, nil
}
