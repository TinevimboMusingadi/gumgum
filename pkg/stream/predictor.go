package stream

import (
	"fmt"
)

// ApplyPredictor applies predictor decoding to data.
// Predictors are used to improve compression ratios for image data.
func ApplyPredictor(data []byte, params DecodeParams) ([]byte, error) {
	predictor := params.Predictor
	
	if predictor == 1 {
		return data, nil // No predictor
	}
	
	if predictor == 2 {
		return applyTIFFPredictor(data, params)
	}
	
	if predictor >= 10 && predictor <= 15 {
		return applyPNGPredictor(data, params, predictor)
	}
	
	return nil, fmt.Errorf("unsupported predictor: %d", predictor)
}

// applyTIFFPredictor applies TIFF predictor 2 (horizontal differencing).
func applyTIFFPredictor(data []byte, params DecodeParams) ([]byte, error) {
	columns := params.Columns
	colors := params.Colors
	bpc := params.BitsPerComponent
	
	if columns == 0 {
		columns = 1
	}
	if colors == 0 {
		colors = 1
	}
	if bpc == 0 {
		bpc = 8
	}
	
	// Only support 8-bit components for now
	if bpc != 8 {
		return data, nil
	}
	
	bytesPerRow := columns * colors
	if len(data) == 0 || bytesPerRow == 0 {
		return data, nil
	}
	
	numRows := len(data) / bytesPerRow
	result := make([]byte, len(data))
	copy(result, data)
	
	for row := 0; row < numRows; row++ {
		rowStart := row * bytesPerRow
		for col := colors; col < bytesPerRow; col++ {
			result[rowStart+col] += result[rowStart+col-colors]
		}
	}
	
	return result, nil
}

// applyPNGPredictor applies PNG-style predictors (10-15).
func applyPNGPredictor(data []byte, params DecodeParams, predictor int) ([]byte, error) {
	columns := params.Columns
	colors := params.Colors
	bpc := params.BitsPerComponent
	
	if columns == 0 {
		columns = 1
	}
	if colors == 0 {
		colors = 1
	}
	if bpc == 0 {
		bpc = 8
	}
	
	// Calculate bytes per pixel and row
	bytesPerPixel := (colors * bpc + 7) / 8
	rowSize := (columns*colors*bpc + 7) / 8
	
	// For PNG predictors, each row has a filter byte prefix
	inputRowSize := rowSize + 1
	
	if len(data) == 0 || inputRowSize == 0 {
		return data, nil
	}
	
	numRows := len(data) / inputRowSize
	if numRows == 0 {
		// Data might not have filter bytes (optimum predictor)
		return applyOptimumPredictor(data, params)
	}
	
	result := make([]byte, 0, numRows*rowSize)
	prevRow := make([]byte, rowSize)
	
	for row := 0; row < numRows; row++ {
		start := row * inputRowSize
		if start >= len(data) {
			break
		}
		
		// Get filter type for this row
		filterType := data[start]
		
		// Handle per-row filter vs fixed filter
		if predictor >= 10 && predictor <= 14 {
			// Fixed filter type (predictor - 10 = filter type)
			// But if data has filter bytes, use them
			if filterType > 4 {
				// No filter byte, use fixed filter
				filterType = byte(predictor - 10)
				start-- // Adjust for no filter byte
			}
		}
		
		// Get row data (after filter byte)
		rowDataStart := start + 1
		rowDataEnd := rowDataStart + rowSize
		if rowDataEnd > len(data) {
			rowDataEnd = len(data)
		}
		
		rowData := make([]byte, rowSize)
		if rowDataStart < len(data) {
			copy(rowData, data[rowDataStart:rowDataEnd])
		}
		
		// Apply the appropriate filter
		decoded := applyPNGFilter(rowData, prevRow, filterType, bytesPerPixel)
		
		result = append(result, decoded...)
		copy(prevRow, decoded)
	}
	
	return result, nil
}

// applyOptimumPredictor tries to decode without row filter bytes.
func applyOptimumPredictor(data []byte, params DecodeParams) ([]byte, error) {
	// Assume no filter bytes, just return data
	return data, nil
}

// applyPNGFilter applies a single PNG filter to a row.
func applyPNGFilter(row, prevRow []byte, filterType byte, bytesPerPixel int) []byte {
	decoded := make([]byte, len(row))
	copy(decoded, row)
	
	switch filterType {
	case 0: // None
		// No change needed
		
	case 1: // Sub
		for i := bytesPerPixel; i < len(decoded); i++ {
			decoded[i] += decoded[i-bytesPerPixel]
		}
		
	case 2: // Up
		for i := 0; i < len(decoded) && i < len(prevRow); i++ {
			decoded[i] += prevRow[i]
		}
		
	case 3: // Average
		for i := 0; i < len(decoded); i++ {
			var left, up byte
			if i >= bytesPerPixel {
				left = decoded[i-bytesPerPixel]
			}
			if i < len(prevRow) {
				up = prevRow[i]
			}
			decoded[i] += byte((int(left) + int(up)) / 2)
		}
		
	case 4: // Paeth
		for i := 0; i < len(decoded); i++ {
			var a, b, c byte
			if i >= bytesPerPixel {
				a = decoded[i-bytesPerPixel]
			}
			if i < len(prevRow) {
				b = prevRow[i]
			}
			if i >= bytesPerPixel && i-bytesPerPixel < len(prevRow) {
				c = prevRow[i-bytesPerPixel]
			}
			decoded[i] += paeth(a, b, c)
		}
	}
	
	return decoded
}

// paeth implements the Paeth predictor function.
func paeth(a, b, c byte) byte {
	// a = left, b = above, c = upper left
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

// EncodePNGPredictor applies PNG predictor encoding (for creating PDFs).
func EncodePNGPredictor(data []byte, params DecodeParams) ([]byte, error) {
	columns := params.Columns
	colors := params.Colors
	bpc := params.BitsPerComponent
	
	if columns == 0 {
		columns = 1
	}
	if colors == 0 {
		colors = 1
	}
	if bpc == 0 {
		bpc = 8
	}
	
	bytesPerPixel := (colors * bpc + 7) / 8
	rowSize := (columns*colors*bpc + 7) / 8
	
	if len(data) == 0 || rowSize == 0 {
		return data, nil
	}
	
	numRows := len(data) / rowSize
	result := make([]byte, 0, numRows*(rowSize+1))
	prevRow := make([]byte, rowSize)
	
	for row := 0; row < numRows; row++ {
		start := row * rowSize
		end := start + rowSize
		if end > len(data) {
			end = len(data)
		}
		
		rowData := data[start:end]
		
		// Use Sub filter (type 1) - simple and often effective
		encoded := make([]byte, len(rowData))
		for i := 0; i < len(rowData); i++ {
			if i < bytesPerPixel {
				encoded[i] = rowData[i]
			} else {
				encoded[i] = rowData[i] - rowData[i-bytesPerPixel]
			}
		}
		
		// Add filter byte
		result = append(result, 1) // Sub filter
		result = append(result, encoded...)
		
		copy(prevRow, rowData)
	}
	
	return result, nil
}
