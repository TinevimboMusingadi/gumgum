package cos

import (
	"fmt"
)

// Object represents any PDF object.
type Object interface {
	String() string
}

// Null represents the PDF null object.
type Null struct{}

func (n Null) String() string { return "null" }

// Boolean represents a PDF boolean value.
type Boolean bool

func (b Boolean) String() string {
	if b {
		return "true"
	}
	return "false"
}

// Integer represents a PDF integer.
type Integer int64

func (i Integer) String() string { return fmt.Sprintf("%d", i) }

// Real represents a PDF real number.
type Real float64

func (r Real) String() string { return fmt.Sprintf("%g", r) }

// String represents a PDF string (literal or hex-encoded).
type String string

func (s String) String() string { return fmt.Sprintf("(%s)", string(s)) }

// Name represents a PDF name object (e.g., /Type, /Font).
type Name string

func (n Name) String() string { return "/" + string(n) }

// Array represents a PDF array.
type Array []Object

func (a Array) String() string {
	result := "["
	for i, obj := range a {
		if i > 0 {
			result += " "
		}
		result += obj.String()
	}
	return result + "]"
}

// Dict represents a PDF dictionary.
type Dict map[Name]Object

func (d Dict) String() string {
	result := "<<"
	for k, v := range d {
		result += fmt.Sprintf(" %s %s", k.String(), v.String())
	}
	return result + " >>"
}

// Get returns a value from the dictionary, or nil if not found.
func (d Dict) Get(key string) Object {
	return d[Name(key)]
}

// GetName returns a Name value from the dictionary.
func (d Dict) GetName(key string) (Name, bool) {
	if obj, ok := d[Name(key)]; ok {
		if n, ok := obj.(Name); ok {
			return n, true
		}
	}
	return "", false
}

// GetInt returns an Integer value from the dictionary.
func (d Dict) GetInt(key string) (int64, bool) {
	if obj, ok := d[Name(key)]; ok {
		if n, ok := obj.(Integer); ok {
			return int64(n), true
		}
	}
	return 0, false
}

// GetReal returns a Real or Integer value as float64.
func (d Dict) GetReal(key string) (float64, bool) {
	if obj, ok := d[Name(key)]; ok {
		switch v := obj.(type) {
		case Real:
			return float64(v), true
		case Integer:
			return float64(v), true
		}
	}
	return 0, false
}

// GetArray returns an Array value from the dictionary.
func (d Dict) GetArray(key string) (Array, bool) {
	if obj, ok := d[Name(key)]; ok {
		if arr, ok := obj.(Array); ok {
			return arr, true
		}
	}
	return nil, false
}

// GetDict returns a Dict value from the dictionary.
func (d Dict) GetDict(key string) (Dict, bool) {
	if obj, ok := d[Name(key)]; ok {
		if dict, ok := obj.(Dict); ok {
			return dict, true
		}
	}
	return nil, false
}

// GetRef returns a Reference value from the dictionary.
func (d Dict) GetRef(key string) (*Reference, bool) {
	if obj, ok := d[Name(key)]; ok {
		if ref, ok := obj.(*Reference); ok {
			return ref, true
		}
	}
	return nil, false
}

// Reference represents an indirect object reference (e.g., 5 0 R).
type Reference struct {
	ObjectNumber     int
	GenerationNumber int
}

func (r *Reference) String() string {
	return fmt.Sprintf("%d %d R", r.ObjectNumber, r.GenerationNumber)
}

// Stream represents a PDF stream with a dictionary and raw data.
type Stream struct {
	Dict Dict
	Data []byte
}

func (s *Stream) String() string {
	return fmt.Sprintf("%s stream[%d bytes]", s.Dict.String(), len(s.Data))
}

// IndirectObject represents a numbered PDF object (e.g., 5 0 obj ... endobj).
type IndirectObject struct {
	ObjectNumber     int
	GenerationNumber int
	Object           Object
}

func (i *IndirectObject) String() string {
	return fmt.Sprintf("%d %d obj %s endobj", i.ObjectNumber, i.GenerationNumber, i.Object.String())
}
