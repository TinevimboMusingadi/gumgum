// Package cos implements the Carousel Object System (COS) parser for PDF files.
// COS is the low-level syntax that PDF is built on - a graph of objects that
// can reference each other by ID.
package cos

// TokenType represents the type of a PDF token.
type TokenType int

const (
	TokenEOF TokenType = iota
	TokenError

	// Delimiters
	TokenDictBegin    // <<
	TokenDictEnd      // >>
	TokenArrayBegin   // [
	TokenArrayEnd     // ]
	TokenStringBegin  // (
	TokenHexBegin     // <
	TokenComment      // %

	// Values
	TokenName    // /Name
	TokenNumber  // 123, 3.14, -5
	TokenString  // (Hello World) or <48656C6C6F>
	TokenBoolean // true, false
	TokenNull    // null

	// Keywords
	TokenObj       // obj
	TokenEndObj    // endobj
	TokenStream    // stream
	TokenEndStream // endstream
	TokenXref      // xref
	TokenTrailer   // trailer
	TokenStartXref // startxref
	TokenR         // R (reference)
)

// Token represents a single token from the PDF lexer.
type Token struct {
	Type    TokenType
	Value   string
	Int     int64
	Float   float64
	IsFloat bool
	Pos     int64 // Byte position in file
}

// String returns a human-readable representation of the token type.
func (t TokenType) String() string {
	names := map[TokenType]string{
		TokenEOF:        "EOF",
		TokenError:      "ERROR",
		TokenDictBegin:  "DICT_BEGIN",
		TokenDictEnd:    "DICT_END",
		TokenArrayBegin: "ARRAY_BEGIN",
		TokenArrayEnd:   "ARRAY_END",
		TokenName:       "NAME",
		TokenNumber:     "NUMBER",
		TokenString:     "STRING",
		TokenBoolean:    "BOOLEAN",
		TokenNull:       "NULL",
		TokenObj:        "OBJ",
		TokenEndObj:     "ENDOBJ",
		TokenStream:     "STREAM",
		TokenEndStream:  "ENDSTREAM",
		TokenXref:       "XREF",
		TokenTrailer:    "TRAILER",
		TokenStartXref:  "STARTXREF",
		TokenR:          "R",
	}
	if name, ok := names[t]; ok {
		return name
	}
	return "UNKNOWN"
}
