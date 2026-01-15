package cos

import (
	"bytes"
	"fmt"
	"io"
	"strconv"
)

// Lexer tokenizes PDF content into a stream of tokens.
type Lexer struct {
	data []byte
	pos  int
	size int
}

// NewLexer creates a new lexer from a byte slice.
func NewLexer(data []byte) *Lexer {
	return &Lexer{
		data: data,
		pos:  0,
		size: len(data),
	}
}

// NewLexerFromReader creates a new lexer by reading all data from a reader.
func NewLexerFromReader(r io.Reader) (*Lexer, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return NewLexer(data), nil
}

// Position returns the current byte position.
func (l *Lexer) Position() int {
	return l.pos
}

// SetPosition sets the lexer position.
func (l *Lexer) SetPosition(pos int) {
	if pos >= 0 && pos <= l.size {
		l.pos = pos
	}
}

// peek returns the byte at current position without advancing.
func (l *Lexer) peek() (byte, bool) {
	if l.pos >= l.size {
		return 0, false
	}
	return l.data[l.pos], true
}

// peekN returns up to n bytes from current position without advancing.
func (l *Lexer) peekN(n int) []byte {
	end := l.pos + n
	if end > l.size {
		end = l.size
	}
	return l.data[l.pos:end]
}

// advance moves forward by one byte.
func (l *Lexer) advance() byte {
	if l.pos >= l.size {
		return 0
	}
	b := l.data[l.pos]
	l.pos++
	return b
}

// skipWhitespace skips PDF whitespace characters (space, tab, CR, LF, FF, NUL).
func (l *Lexer) skipWhitespace() {
	for l.pos < l.size {
		b := l.data[l.pos]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == 0 {
			l.pos++
		} else if b == '%' {
			// Skip comment until end of line
			l.skipComment()
		} else {
			break
		}
	}
}

// skipComment skips from % to end of line.
func (l *Lexer) skipComment() {
	for l.pos < l.size {
		b := l.data[l.pos]
		l.pos++
		if b == '\r' || b == '\n' {
			break
		}
	}
}

// isDelimiter returns true if b is a PDF delimiter.
func isDelimiter(b byte) bool {
	return b == '(' || b == ')' || b == '<' || b == '>' ||
		b == '[' || b == ']' || b == '{' || b == '}' ||
		b == '/' || b == '%'
}

// isWhitespace returns true if b is PDF whitespace.
func isWhitespace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n' || b == '\f' || b == 0
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	l.skipWhitespace()

	if l.pos >= l.size {
		return Token{Type: TokenEOF, Pos: int64(l.pos)}
	}

	startPos := l.pos
	b := l.data[l.pos]

	// Two-character tokens
	if l.pos+1 < l.size {
		two := string(l.data[l.pos : l.pos+2])
		switch two {
		case "<<":
			l.pos += 2
			return Token{Type: TokenDictBegin, Value: "<<", Pos: int64(startPos)}
		case ">>":
			l.pos += 2
			return Token{Type: TokenDictEnd, Value: ">>", Pos: int64(startPos)}
		}
	}

	// Single character tokens
	switch b {
	case '[':
		l.pos++
		return Token{Type: TokenArrayBegin, Value: "[", Pos: int64(startPos)}
	case ']':
		l.pos++
		return Token{Type: TokenArrayEnd, Value: "]", Pos: int64(startPos)}
	case '/':
		return l.scanName()
	case '(':
		return l.scanLiteralString()
	case '<':
		return l.scanHexString()
	}

	// Numbers (including negative)
	if (b >= '0' && b <= '9') || b == '-' || b == '+' || b == '.' {
		return l.scanNumber()
	}

	// Keywords and booleans
	if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
		return l.scanKeyword()
	}

	// Unknown character
	l.pos++
	return Token{Type: TokenError, Value: fmt.Sprintf("unexpected character: %c", b), Pos: int64(startPos)}
}

// scanName scans a PDF name (e.g., /Type, /Font).
func (l *Lexer) scanName() Token {
	startPos := l.pos
	l.pos++ // Skip '/'

	var buf bytes.Buffer
	for l.pos < l.size {
		b := l.data[l.pos]
		if isWhitespace(b) || isDelimiter(b) {
			break
		}
		// Handle #XX hex escape sequences
		if b == '#' && l.pos+2 < l.size {
			hex := string(l.data[l.pos+1 : l.pos+3])
			if val, err := strconv.ParseUint(hex, 16, 8); err == nil {
				buf.WriteByte(byte(val))
				l.pos += 3
				continue
			}
		}
		buf.WriteByte(b)
		l.pos++
	}

	return Token{Type: TokenName, Value: buf.String(), Pos: int64(startPos)}
}

// scanLiteralString scans a parentheses-delimited string, handling escapes and nested parens.
func (l *Lexer) scanLiteralString() Token {
	startPos := l.pos
	l.pos++ // Skip '('

	var buf bytes.Buffer
	depth := 1

	for l.pos < l.size && depth > 0 {
		b := l.data[l.pos]

		if b == '\\' && l.pos+1 < l.size {
			l.pos++
			escaped := l.data[l.pos]
			switch escaped {
			case 'n':
				buf.WriteByte('\n')
			case 'r':
				buf.WriteByte('\r')
			case 't':
				buf.WriteByte('\t')
			case 'b':
				buf.WriteByte('\b')
			case 'f':
				buf.WriteByte('\f')
			case '(':
				buf.WriteByte('(')
			case ')':
				buf.WriteByte(')')
			case '\\':
				buf.WriteByte('\\')
			case '\r':
				// Escaped line break - skip
				if l.pos+1 < l.size && l.data[l.pos+1] == '\n' {
					l.pos++
				}
			case '\n':
				// Escaped line break - skip
			default:
				// Octal escape?
				if escaped >= '0' && escaped <= '7' {
					octal := string(escaped)
					for i := 0; i < 2 && l.pos+1 < l.size; i++ {
						next := l.data[l.pos+1]
						if next >= '0' && next <= '7' {
							octal += string(next)
							l.pos++
						} else {
							break
						}
					}
					if val, err := strconv.ParseUint(octal, 8, 8); err == nil {
						buf.WriteByte(byte(val))
					}
				} else {
					buf.WriteByte(escaped)
				}
			}
			l.pos++
			continue
		}

		if b == '(' {
			depth++
			buf.WriteByte(b)
		} else if b == ')' {
			depth--
			if depth > 0 {
				buf.WriteByte(b)
			}
		} else {
			buf.WriteByte(b)
		}
		l.pos++
	}

	return Token{Type: TokenString, Value: buf.String(), Pos: int64(startPos)}
}

// scanHexString scans a hex-encoded string <48656C6C6F>.
func (l *Lexer) scanHexString() Token {
	startPos := l.pos
	l.pos++ // Skip '<'

	var hexBuf bytes.Buffer
	for l.pos < l.size {
		b := l.data[l.pos]
		if b == '>' {
			l.pos++
			break
		}
		if !isWhitespace(b) {
			hexBuf.WriteByte(b)
		}
		l.pos++
	}

	// Decode hex to bytes
	hexStr := hexBuf.String()
	if len(hexStr)%2 == 1 {
		hexStr += "0" // Pad with trailing 0
	}

	var result bytes.Buffer
	for i := 0; i < len(hexStr); i += 2 {
		if val, err := strconv.ParseUint(hexStr[i:i+2], 16, 8); err == nil {
			result.WriteByte(byte(val))
		}
	}

	return Token{Type: TokenString, Value: result.String(), Pos: int64(startPos)}
}

// scanNumber scans an integer or real number.
func (l *Lexer) scanNumber() Token {
	startPos := l.pos
	var buf bytes.Buffer
	hasDecimal := false

	// Handle sign
	if b, ok := l.peek(); ok && (b == '+' || b == '-') {
		buf.WriteByte(l.advance())
	}

	for l.pos < l.size {
		b := l.data[l.pos]
		if b >= '0' && b <= '9' {
			buf.WriteByte(b)
			l.pos++
		} else if b == '.' && !hasDecimal {
			hasDecimal = true
			buf.WriteByte(b)
			l.pos++
		} else {
			break
		}
	}

	numStr := buf.String()
	tok := Token{Type: TokenNumber, Value: numStr, Pos: int64(startPos)}

	if hasDecimal {
		tok.IsFloat = true
		tok.Float, _ = strconv.ParseFloat(numStr, 64)
	} else {
		tok.Int, _ = strconv.ParseInt(numStr, 10, 64)
		tok.Float = float64(tok.Int)
	}

	return tok
}

// scanKeyword scans a keyword like obj, endobj, true, false, null.
func (l *Lexer) scanKeyword() Token {
	startPos := l.pos
	var buf bytes.Buffer

	for l.pos < l.size {
		b := l.data[l.pos]
		if (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') {
			buf.WriteByte(b)
			l.pos++
		} else {
			break
		}
	}

	word := buf.String()
	tok := Token{Value: word, Pos: int64(startPos)}

	switch word {
	case "obj":
		tok.Type = TokenObj
	case "endobj":
		tok.Type = TokenEndObj
	case "stream":
		tok.Type = TokenStream
	case "endstream":
		tok.Type = TokenEndStream
	case "xref":
		tok.Type = TokenXref
	case "trailer":
		tok.Type = TokenTrailer
	case "startxref":
		tok.Type = TokenStartXref
	case "true":
		tok.Type = TokenBoolean
		tok.Int = 1
	case "false":
		tok.Type = TokenBoolean
		tok.Int = 0
	case "null":
		tok.Type = TokenNull
	case "R":
		tok.Type = TokenR
	default:
		tok.Type = TokenError
		tok.Value = fmt.Sprintf("unknown keyword: %s", word)
	}

	return tok
}

// PeekToken returns the next token without consuming it.
func (l *Lexer) PeekToken() Token {
	pos := l.pos
	tok := l.NextToken()
	l.pos = pos
	return tok
}
