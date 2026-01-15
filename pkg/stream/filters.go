// Package stream provides PDF stream filter implementations.
// Streams in PDF can be compressed using various filters like FlateDecode,
// ASCIIHexDecode, ASCII85Decode, LZWDecode, etc.
package stream

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// Filter represents a stream filter type.
type Filter string

const (
	FilterFlateDecode    Filter = "FlateDecode"
	FilterASCIIHexDecode Filter = "ASCIIHexDecode"
	FilterASCII85Decode  Filter = "ASCII85Decode"
	FilterLZWDecode      Filter = "LZWDecode"
	FilterRunLengthDecode Filter = "RunLengthDecode"
	FilterDCTDecode      Filter = "DCTDecode" // JPEG
	FilterJPXDecode      Filter = "JPXDecode" // JPEG2000
	FilterCCITTFaxDecode Filter = "CCITTFaxDecode"
)

// DecodeParams holds common decode parameters.
type DecodeParams struct {
	Predictor        int
	Colors           int
	BitsPerComponent int
	Columns          int
	EarlyChange      int // For LZW
}

// DefaultDecodeParams returns default decode parameters.
func DefaultDecodeParams() DecodeParams {
	return DecodeParams{
		Predictor:        1,
		Colors:           1,
		BitsPerComponent: 8,
		Columns:          1,
		EarlyChange:      1,
	}
}

// Decode applies a filter to decode data.
func Decode(data []byte, filter Filter, params DecodeParams) ([]byte, error) {
	var decoded []byte
	var err error

	switch filter {
	case FilterFlateDecode:
		decoded, err = DecodeFlateDecode(data)
	case FilterASCIIHexDecode:
		decoded, err = DecodeASCIIHex(data)
	case FilterASCII85Decode:
		decoded, err = DecodeASCII85(data)
	case FilterLZWDecode:
		decoded, err = DecodeLZW(data, params.EarlyChange)
	case FilterRunLengthDecode:
		decoded, err = DecodeRunLength(data)
	case FilterDCTDecode:
		// JPEG data - pass through (handled by image decoders)
		return data, nil
	case FilterJPXDecode:
		// JPEG2000 data - pass through
		return data, nil
	default:
		return nil, fmt.Errorf("unsupported filter: %s", filter)
	}

	if err != nil {
		return nil, err
	}

	// Apply predictor if needed
	if params.Predictor > 1 {
		decoded, err = ApplyPredictor(decoded, params)
		if err != nil {
			return nil, err
		}
	}

	return decoded, nil
}

// DecodeFlateDecode decompresses zlib-compressed data.
func DecodeFlateDecode(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("zlib error: %w", err)
	}
	defer r.Close()

	decoded, err := io.ReadAll(r)
	if err != nil {
		// Some PDFs have incomplete zlib streams, try to return what we got
		if len(decoded) > 0 {
			return decoded, nil
		}
		return nil, fmt.Errorf("zlib read error: %w", err)
	}

	return decoded, nil
}

// DecodeASCIIHex decodes ASCII hexadecimal encoded data.
func DecodeASCIIHex(data []byte) ([]byte, error) {
	var result []byte
	var hexByte byte
	var haveNibble bool

	for _, b := range data {
		if b == '>' {
			break // End of data marker
		}

		// Skip whitespace
		if isWhitespace(b) {
			continue
		}

		var nibble byte
		switch {
		case b >= '0' && b <= '9':
			nibble = b - '0'
		case b >= 'A' && b <= 'F':
			nibble = b - 'A' + 10
		case b >= 'a' && b <= 'f':
			nibble = b - 'a' + 10
		default:
			continue // Skip invalid characters
		}

		if haveNibble {
			result = append(result, hexByte<<4|nibble)
			haveNibble = false
		} else {
			hexByte = nibble
			haveNibble = true
		}
	}

	// Handle odd number of nibbles
	if haveNibble {
		result = append(result, hexByte<<4)
	}

	return result, nil
}

// DecodeASCII85 decodes ASCII85 (Base85) encoded data.
func DecodeASCII85(data []byte) ([]byte, error) {
	var result []byte
	var tuple uint32
	var count int

	for i := 0; i < len(data); i++ {
		b := data[i]

		// Check for end marker ~>
		if b == '~' {
			break
		}

		// Skip whitespace
		if isWhitespace(b) {
			continue
		}

		// Handle 'z' special case (represents four zero bytes)
		if b == 'z' && count == 0 {
			result = append(result, 0, 0, 0, 0)
			continue
		}

		// Valid ASCII85 characters are '!' (33) to 'u' (117)
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

	// Handle remaining bytes (partial group)
	if count > 0 {
		// Pad with 'u' characters
		for i := count; i < 5; i++ {
			tuple = tuple*85 + 84
		}

		// Output only the bytes we need
		switch count {
		case 2:
			result = append(result, byte(tuple>>24))
		case 3:
			result = append(result, byte(tuple>>24), byte(tuple>>16))
		case 4:
			result = append(result, byte(tuple>>24), byte(tuple>>16), byte(tuple>>8))
		}
	}

	return result, nil
}

// DecodeLZW decodes LZW-compressed data.
func DecodeLZW(data []byte, earlyChange int) ([]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}

	decoder := newLZWDecoder(data, earlyChange == 1)
	return decoder.decode()
}

// lzwDecoder implements LZW decompression for PDF.
type lzwDecoder struct {
	data        []byte
	pos         int
	bitPos      int
	earlyChange bool
	
	table     [][]byte
	codeSize  int
	nextCode  int
	clearCode int
	endCode   int
}

func newLZWDecoder(data []byte, earlyChange bool) *lzwDecoder {
	d := &lzwDecoder{
		data:        data,
		earlyChange: earlyChange,
		clearCode:   256,
		endCode:     257,
	}
	d.reset()
	return d
}

func (d *lzwDecoder) reset() {
	d.table = make([][]byte, 4096)
	for i := 0; i < 256; i++ {
		d.table[i] = []byte{byte(i)}
	}
	d.codeSize = 9
	d.nextCode = 258
}

func (d *lzwDecoder) readCode() (int, bool) {
	if d.pos >= len(d.data) {
		return 0, false
	}

	// Read bits across byte boundaries
	code := 0
	bitsNeeded := d.codeSize

	for bitsNeeded > 0 {
		if d.pos >= len(d.data) {
			return 0, false
		}

		bitsAvail := 8 - d.bitPos
		bitsToRead := bitsNeeded
		if bitsToRead > bitsAvail {
			bitsToRead = bitsAvail
		}

		mask := (1 << bitsToRead) - 1
		shift := bitsAvail - bitsToRead
		bits := (int(d.data[d.pos]) >> shift) & mask

		code = (code << bitsToRead) | bits
		bitsNeeded -= bitsToRead
		d.bitPos += bitsToRead

		if d.bitPos >= 8 {
			d.bitPos = 0
			d.pos++
		}
	}

	return code, true
}

func (d *lzwDecoder) decode() ([]byte, error) {
	var result []byte
	var prevSeq []byte

	for {
		code, ok := d.readCode()
		if !ok {
			break
		}

		if code == d.endCode {
			break
		}

		if code == d.clearCode {
			d.reset()
			prevSeq = nil
			continue
		}

		var seq []byte
		if code < d.nextCode {
			seq = d.table[code]
		} else if code == d.nextCode && prevSeq != nil {
			seq = append(prevSeq, prevSeq[0])
		} else {
			return nil, fmt.Errorf("invalid LZW code: %d", code)
		}

		result = append(result, seq...)

		if prevSeq != nil && d.nextCode < 4096 {
			newEntry := make([]byte, len(prevSeq)+1)
			copy(newEntry, prevSeq)
			newEntry[len(prevSeq)] = seq[0]
			d.table[d.nextCode] = newEntry
			d.nextCode++

			// Adjust code size
			threshold := 1 << d.codeSize
			if d.earlyChange {
				threshold--
			}
			if d.nextCode >= threshold && d.codeSize < 12 {
				d.codeSize++
			}
		}

		prevSeq = seq
	}

	return result, nil
}

// DecodeRunLength decodes run-length encoded data.
func DecodeRunLength(data []byte) ([]byte, error) {
	var result []byte
	i := 0

	for i < len(data) {
		length := int(data[i])
		i++

		if length == 128 {
			// End of data
			break
		} else if length < 128 {
			// Copy next length+1 bytes literally
			count := length + 1
			if i+count > len(data) {
				count = len(data) - i
			}
			result = append(result, data[i:i+count]...)
			i += count
		} else {
			// Repeat next byte 257-length times
			if i >= len(data) {
				break
			}
			repeat := 257 - length
			b := data[i]
			i++
			for j := 0; j < repeat; j++ {
				result = append(result, b)
			}
		}
	}

	return result, nil
}

func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == 0
}
