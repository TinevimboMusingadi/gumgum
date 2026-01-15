package cos

import (
	"bytes"
	"fmt"
)

// Parser parses PDF objects from a token stream.
type Parser struct {
	lexer *Lexer
}


// NewParser creates a new parser from a lexer.
func NewParser(lexer *Lexer) *Parser {
	return &Parser{lexer: lexer}
}

// ParseObject parses any PDF object.
func (p *Parser) ParseObject() (Object, error) {
	tok := p.lexer.NextToken()
	return p.parseObjectFromToken(tok)
}

func (p *Parser) parseObjectFromToken(tok Token) (Object, error) {
	switch tok.Type {
	case TokenEOF:
		return nil, fmt.Errorf("unexpected end of file")
	case TokenError:
		return nil, fmt.Errorf("lexer error: %s", tok.Value)
	case TokenNull:
		return Null{}, nil
	case TokenBoolean:
		return Boolean(tok.Int == 1), nil
	case TokenNumber:
		// Check if this might be a reference (num gen R)
		if !tok.IsFloat {
			// Peek ahead to see if this is a reference
			pos := p.lexer.Position()
			tok2 := p.lexer.NextToken()
			if tok2.Type == TokenNumber && !tok2.IsFloat {
				tok3 := p.lexer.NextToken()
				if tok3.Type == TokenR {
					return &Reference{
						ObjectNumber:     int(tok.Int),
						GenerationNumber: int(tok2.Int),
					}, nil
				}
			}
			// Not a reference, restore position
			p.lexer.SetPosition(pos)
		}
		if tok.IsFloat {
			return Real(tok.Float), nil
		}
		return Integer(tok.Int), nil
	case TokenString:
		return String(tok.Value), nil
	case TokenName:
		return Name(tok.Value), nil
	case TokenArrayBegin:
		return p.parseArray()
	case TokenDictBegin:
		return p.parseDictOrStream()
	default:
		return nil, fmt.Errorf("unexpected token: %s (%s)", tok.Type, tok.Value)
	}
}

// parseArray parses a PDF array.
func (p *Parser) parseArray() (Array, error) {
	var arr Array

	for {
		tok := p.lexer.PeekToken()
		if tok.Type == TokenArrayEnd {
			p.lexer.NextToken() // Consume ]
			break
		}
		if tok.Type == TokenEOF {
			return nil, fmt.Errorf("unexpected end of file in array")
		}

		obj, err := p.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("error parsing array element: %w", err)
		}
		arr = append(arr, obj)
	}

	return arr, nil
}

// parseDictOrStream parses a dictionary, potentially followed by a stream.
func (p *Parser) parseDictOrStream() (Object, error) {
	dict, err := p.parseDict()
	if err != nil {
		return nil, err
	}

	// Check if this is followed by a stream
	p.lexer.skipWhitespace()
	pos := p.lexer.Position()

	// Check for "stream" keyword
	if p.lexer.pos+6 <= p.lexer.size {
		if string(p.lexer.data[p.lexer.pos:p.lexer.pos+6]) == "stream" {
			p.lexer.pos += 6

			// Skip single EOL (CR, LF, or CRLF)
			if p.lexer.pos < p.lexer.size && p.lexer.data[p.lexer.pos] == '\r' {
				p.lexer.pos++
			}
			if p.lexer.pos < p.lexer.size && p.lexer.data[p.lexer.pos] == '\n' {
				p.lexer.pos++
			}

			// Get stream length
			var streamLen int64
			if length, ok := dict.GetInt("Length"); ok {
				streamLen = length
			} else {
				return nil, fmt.Errorf("stream without Length")
			}

			// Read stream data
			streamStart := p.lexer.pos
			streamEnd := streamStart + int(streamLen)
			if streamEnd > p.lexer.size {
				streamEnd = p.lexer.size
			}

			data := p.lexer.data[streamStart:streamEnd]
			p.lexer.pos = streamEnd

			// Skip to endstream
			p.lexer.skipWhitespace()
			if p.lexer.pos+9 <= p.lexer.size && string(p.lexer.data[p.lexer.pos:p.lexer.pos+9]) == "endstream" {
				p.lexer.pos += 9
			}

			return &Stream{Dict: dict, Data: data}, nil
		}
	}

	p.lexer.SetPosition(pos)
	return dict, nil
}

// parseDict parses a PDF dictionary (without checking for stream).
func (p *Parser) parseDict() (Dict, error) {
	dict := make(Dict)

	for {
		tok := p.lexer.PeekToken()
		if tok.Type == TokenDictEnd {
			p.lexer.NextToken() // Consume >>
			break
		}
		if tok.Type == TokenEOF {
			return nil, fmt.Errorf("unexpected end of file in dictionary")
		}

		// Key must be a name
		tok = p.lexer.NextToken()
		if tok.Type != TokenName {
			return nil, fmt.Errorf("expected name as dictionary key, got %s", tok.Type)
		}
		key := Name(tok.Value)

		// Parse value
		value, err := p.ParseObject()
		if err != nil {
			return nil, fmt.Errorf("error parsing dictionary value for key %s: %w", key, err)
		}

		dict[key] = value
	}

	return dict, nil
}

// ParseIndirectObject parses an indirect object (num gen obj ... endobj).
func (p *Parser) ParseIndirectObject() (*IndirectObject, error) {
	// Object number
	tok := p.lexer.NextToken()
	if tok.Type != TokenNumber || tok.IsFloat {
		return nil, fmt.Errorf("expected object number, got %s", tok.Type)
	}
	objNum := int(tok.Int)

	// Generation number
	tok = p.lexer.NextToken()
	if tok.Type != TokenNumber || tok.IsFloat {
		return nil, fmt.Errorf("expected generation number, got %s", tok.Type)
	}
	genNum := int(tok.Int)

	// "obj" keyword
	tok = p.lexer.NextToken()
	if tok.Type != TokenObj {
		return nil, fmt.Errorf("expected 'obj' keyword, got %s", tok.Type)
	}

	// Parse the object content
	obj, err := p.ParseObject()
	if err != nil {
		return nil, fmt.Errorf("error parsing indirect object content: %w", err)
	}

	// "endobj" keyword
	p.lexer.skipWhitespace()
	tok = p.lexer.NextToken()
	if tok.Type != TokenEndObj {
		// Some PDFs have malformed objects, be lenient
		// Try to find endobj
		for tok.Type != TokenEndObj && tok.Type != TokenEOF {
			tok = p.lexer.NextToken()
		}
	}

	return &IndirectObject{
		ObjectNumber:     objNum,
		GenerationNumber: genNum,
		Object:           obj,
	}, nil
}

// ParseObjectAt parses an indirect object at the given byte offset.
func ParseObjectAt(data []byte, offset int64) (*IndirectObject, error) {
	if offset < 0 || int(offset) >= len(data) {
		return nil, fmt.Errorf("offset %d out of range", offset)
	}

	lexer := NewLexer(data[offset:])
	parser := NewParser(lexer)
	return parser.ParseIndirectObject()
}

// ParseObjectsFromStream parses objects from an object stream.
func ParseObjectsFromStream(streamData []byte, dict Dict) (map[int]Object, error) {
	n, ok := dict.GetInt("N")
	if !ok {
		return nil, fmt.Errorf("object stream missing N")
	}

	first, ok := dict.GetInt("First")
	if !ok {
		return nil, fmt.Errorf("object stream missing First")
	}

	// Parse the header: pairs of (objNum, offset)
	headerLexer := NewLexer(streamData[:first])
	var entries []struct {
		objNum int
		offset int
	}

	for i := int64(0); i < n; i++ {
		objTok := headerLexer.NextToken()
		if objTok.Type != TokenNumber {
			break
		}
		offTok := headerLexer.NextToken()
		if offTok.Type != TokenNumber {
			break
		}
		entries = append(entries, struct {
			objNum int
			offset int
		}{
			objNum: int(objTok.Int),
			offset: int(offTok.Int),
		})
	}

	// Parse each object
	objects := make(map[int]Object)
	objectsData := streamData[first:]

	for i, entry := range entries {
		var end int
		if i+1 < len(entries) {
			end = entries[i+1].offset
		} else {
			end = len(objectsData)
		}

		if entry.offset >= len(objectsData) {
			continue
		}
		if end > len(objectsData) {
			end = len(objectsData)
		}

		objData := objectsData[entry.offset:end]
		objData = bytes.TrimSpace(objData)

		lexer := NewLexer(objData)
		parser := NewParser(lexer)
		obj, err := parser.ParseObject()
		if err == nil {
			objects[entry.objNum] = obj
		}
	}

	return objects, nil
}
