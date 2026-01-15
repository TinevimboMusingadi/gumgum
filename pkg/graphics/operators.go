package graphics

import (
	"fmt"
	"strconv"
	"strings"
)

// Operator represents a PDF graphics operator.
type Operator struct {
	Name     string
	Operands []interface{}
}

// Interpreter executes PDF graphics operators.
type Interpreter struct {
	stack     *StateStack
	path      *Path
	Resources Resources
	
	// Callbacks for rendering
	OnFill     func(path *Path, state *State, rule FillRule)
	OnStroke   func(path *Path, state *State)
	OnClip     func(path *Path, rule FillRule)
	OnText     func(text string, state *State)
	OnImage    func(name string, state *State)
}

// Resources holds page resources (fonts, images, etc.)
type Resources struct {
	Fonts    map[string]interface{}
	XObjects map[string]interface{}
	ExtGState map[string]interface{}
	ColorSpaces map[string]interface{}
	Patterns  map[string]interface{}
}

// NewInterpreter creates a new graphics interpreter.
func NewInterpreter() *Interpreter {
	return &Interpreter{
		stack: NewStateStack(),
		path:  NewPath(),
		Resources: Resources{
			Fonts:     make(map[string]interface{}),
			XObjects:  make(map[string]interface{}),
			ExtGState: make(map[string]interface{}),
		},
	}
}

// State returns the current graphics state.
func (i *Interpreter) State() *State {
	return i.stack.Current()
}

// Path returns the current path.
func (i *Interpreter) Path() *Path {
	return i.path
}

// Execute runs a list of operators.
func (i *Interpreter) Execute(ops []Operator) error {
	for _, op := range ops {
		if err := i.executeOp(op); err != nil {
			// Log error but continue
			fmt.Printf("Warning: operator %s: %v\n", op.Name, err)
		}
	}
	return nil
}

// executeOp executes a single operator.
func (i *Interpreter) executeOp(op Operator) error {
	state := i.stack.Current()
	
	switch op.Name {
	// Graphics state operators
	case "q":
		i.stack.Push()
	case "Q":
		i.stack.Pop()
	case "cm":
		if len(op.Operands) >= 6 {
			m := Matrix{
				toFloat(op.Operands[0]),
				toFloat(op.Operands[1]),
				toFloat(op.Operands[2]),
				toFloat(op.Operands[3]),
				toFloat(op.Operands[4]),
				toFloat(op.Operands[5]),
			}
			state.CTM = state.CTM.Multiply(m)
		}
	case "w":
		if len(op.Operands) >= 1 {
			state.LineWidth = toFloat(op.Operands[0])
		}
	case "J":
		if len(op.Operands) >= 1 {
			state.LineCap = LineCap(toInt(op.Operands[0]))
		}
	case "j":
		if len(op.Operands) >= 1 {
			state.LineJoin = LineJoin(toInt(op.Operands[0]))
		}
	case "M":
		if len(op.Operands) >= 1 {
			state.MiterLimit = toFloat(op.Operands[0])
		}
	case "d":
		if len(op.Operands) >= 2 {
			if arr, ok := op.Operands[0].([]interface{}); ok {
				state.DashPattern = make([]float64, len(arr))
				for j, v := range arr {
					state.DashPattern[j] = toFloat(v)
				}
			}
			state.DashPhase = toFloat(op.Operands[1])
		}
	case "ri":
		if len(op.Operands) >= 1 {
			state.RenderingIntent = toString(op.Operands[0])
		}
	case "i":
		if len(op.Operands) >= 1 {
			state.Flatness = toFloat(op.Operands[0])
		}
	case "gs":
		if len(op.Operands) >= 1 {
			i.applyExtGState(toString(op.Operands[0]))
		}
		
	// Path construction operators
	case "m":
		if len(op.Operands) >= 2 {
			x, y := toFloat(op.Operands[0]), toFloat(op.Operands[1])
			i.path.MoveTo(x, y)
		}
	case "l":
		if len(op.Operands) >= 2 {
			x, y := toFloat(op.Operands[0]), toFloat(op.Operands[1])
			i.path.LineTo(x, y)
		}
	case "c":
		if len(op.Operands) >= 6 {
			i.path.CurveTo(
				toFloat(op.Operands[0]), toFloat(op.Operands[1]),
				toFloat(op.Operands[2]), toFloat(op.Operands[3]),
				toFloat(op.Operands[4]), toFloat(op.Operands[5]),
			)
		}
	case "v":
		if len(op.Operands) >= 4 {
			i.path.CurveToV(
				toFloat(op.Operands[0]), toFloat(op.Operands[1]),
				toFloat(op.Operands[2]), toFloat(op.Operands[3]),
			)
		}
	case "y":
		if len(op.Operands) >= 4 {
			i.path.CurveToY(
				toFloat(op.Operands[0]), toFloat(op.Operands[1]),
				toFloat(op.Operands[2]), toFloat(op.Operands[3]),
			)
		}
	case "h":
		i.path.Close()
	case "re":
		if len(op.Operands) >= 4 {
			i.path.Rect(
				toFloat(op.Operands[0]), toFloat(op.Operands[1]),
				toFloat(op.Operands[2]), toFloat(op.Operands[3]),
			)
		}
		
	// Path painting operators
	case "S":
		if i.OnStroke != nil {
			i.OnStroke(i.path.Transform(state.CTM), state)
		}
		i.path.Clear()
	case "s":
		i.path.Close()
		if i.OnStroke != nil {
			i.OnStroke(i.path.Transform(state.CTM), state)
		}
		i.path.Clear()
	case "f", "F":
		if i.OnFill != nil {
			i.OnFill(i.path.Transform(state.CTM), state, FillRuleNonZero)
		}
		i.path.Clear()
	case "f*":
		if i.OnFill != nil {
			i.OnFill(i.path.Transform(state.CTM), state, FillRuleEvenOdd)
		}
		i.path.Clear()
	case "B":
		if i.OnFill != nil {
			i.OnFill(i.path.Transform(state.CTM), state, FillRuleNonZero)
		}
		if i.OnStroke != nil {
			i.OnStroke(i.path.Transform(state.CTM), state)
		}
		i.path.Clear()
	case "B*":
		if i.OnFill != nil {
			i.OnFill(i.path.Transform(state.CTM), state, FillRuleEvenOdd)
		}
		if i.OnStroke != nil {
			i.OnStroke(i.path.Transform(state.CTM), state)
		}
		i.path.Clear()
	case "b":
		i.path.Close()
		if i.OnFill != nil {
			i.OnFill(i.path.Transform(state.CTM), state, FillRuleNonZero)
		}
		if i.OnStroke != nil {
			i.OnStroke(i.path.Transform(state.CTM), state)
		}
		i.path.Clear()
	case "b*":
		i.path.Close()
		if i.OnFill != nil {
			i.OnFill(i.path.Transform(state.CTM), state, FillRuleEvenOdd)
		}
		if i.OnStroke != nil {
			i.OnStroke(i.path.Transform(state.CTM), state)
		}
		i.path.Clear()
	case "n":
		i.path.Clear()
		
	// Clipping operators
	case "W":
		if i.OnClip != nil {
			i.OnClip(i.path, FillRuleNonZero)
		}
		state.ClipPath = i.path.Clone()
	case "W*":
		if i.OnClip != nil {
			i.OnClip(i.path, FillRuleEvenOdd)
		}
		state.ClipPath = i.path.Clone()
		
	// Color operators
	case "CS":
		if len(op.Operands) >= 1 {
			state.StrokeColorSpace = ColorSpace(toString(op.Operands[0]))
		}
	case "cs":
		if len(op.Operands) >= 1 {
			state.FillColorSpace = ColorSpace(toString(op.Operands[0]))
		}
	case "SC", "SCN":
		state.StrokeColor = i.parseColor(state.StrokeColorSpace, op.Operands)
	case "sc", "scn":
		state.FillColor = i.parseColor(state.FillColorSpace, op.Operands)
	case "G":
		if len(op.Operands) >= 1 {
			state.StrokeColorSpace = ColorSpaceDeviceGray
			state.StrokeColor = NewGray(toFloat(op.Operands[0]))
		}
	case "g":
		if len(op.Operands) >= 1 {
			state.FillColorSpace = ColorSpaceDeviceGray
			state.FillColor = NewGray(toFloat(op.Operands[0]))
		}
	case "RG":
		if len(op.Operands) >= 3 {
			state.StrokeColorSpace = ColorSpaceDeviceRGB
			state.StrokeColor = NewRGB(
				toFloat(op.Operands[0]),
				toFloat(op.Operands[1]),
				toFloat(op.Operands[2]),
			)
		}
	case "rg":
		if len(op.Operands) >= 3 {
			state.FillColorSpace = ColorSpaceDeviceRGB
			state.FillColor = NewRGB(
				toFloat(op.Operands[0]),
				toFloat(op.Operands[1]),
				toFloat(op.Operands[2]),
			)
		}
	case "K":
		if len(op.Operands) >= 4 {
			state.StrokeColorSpace = ColorSpaceCMYK
			state.StrokeColor = NewCMYK(
				toFloat(op.Operands[0]),
				toFloat(op.Operands[1]),
				toFloat(op.Operands[2]),
				toFloat(op.Operands[3]),
			)
		}
	case "k":
		if len(op.Operands) >= 4 {
			state.FillColorSpace = ColorSpaceCMYK
			state.FillColor = NewCMYK(
				toFloat(op.Operands[0]),
				toFloat(op.Operands[1]),
				toFloat(op.Operands[2]),
				toFloat(op.Operands[3]),
			)
		}
		
	// Text operators
	case "BT":
		state.TextState.TextMatrix = Identity()
		state.TextState.LineMatrix = Identity()
	case "ET":
		// End text object
	case "Tc":
		if len(op.Operands) >= 1 {
			state.TextState.CharSpace = toFloat(op.Operands[0])
		}
	case "Tw":
		if len(op.Operands) >= 1 {
			state.TextState.WordSpace = toFloat(op.Operands[0])
		}
	case "Tz":
		if len(op.Operands) >= 1 {
			state.TextState.HScale = toFloat(op.Operands[0])
		}
	case "TL":
		if len(op.Operands) >= 1 {
			state.TextState.Leading = toFloat(op.Operands[0])
		}
	case "Tf":
		if len(op.Operands) >= 2 {
			state.TextState.FontName = toString(op.Operands[0])
			state.TextState.FontSize = toFloat(op.Operands[1])
		}
	case "Tr":
		if len(op.Operands) >= 1 {
			state.TextState.RenderMode = TextRenderMode(toInt(op.Operands[0]))
		}
	case "Ts":
		if len(op.Operands) >= 1 {
			state.TextState.Rise = toFloat(op.Operands[0])
		}
	case "Td":
		if len(op.Operands) >= 2 {
			tx, ty := toFloat(op.Operands[0]), toFloat(op.Operands[1])
			state.TextState.LineMatrix = Translate(tx, ty).Multiply(state.TextState.LineMatrix)
			state.TextState.TextMatrix = state.TextState.LineMatrix
		}
	case "TD":
		if len(op.Operands) >= 2 {
			tx, ty := toFloat(op.Operands[0]), toFloat(op.Operands[1])
			state.TextState.Leading = -ty
			state.TextState.LineMatrix = Translate(tx, ty).Multiply(state.TextState.LineMatrix)
			state.TextState.TextMatrix = state.TextState.LineMatrix
		}
	case "Tm":
		if len(op.Operands) >= 6 {
			state.TextState.TextMatrix = Matrix{
				toFloat(op.Operands[0]),
				toFloat(op.Operands[1]),
				toFloat(op.Operands[2]),
				toFloat(op.Operands[3]),
				toFloat(op.Operands[4]),
				toFloat(op.Operands[5]),
			}
			state.TextState.LineMatrix = state.TextState.TextMatrix
		}
	case "T*":
		state.TextState.LineMatrix = Translate(0, -state.TextState.Leading).Multiply(state.TextState.LineMatrix)
		state.TextState.TextMatrix = state.TextState.LineMatrix
	case "Tj":
		if len(op.Operands) >= 1 {
			if i.OnText != nil {
				i.OnText(toString(op.Operands[0]), state)
			}
		}
	case "TJ":
		if len(op.Operands) >= 1 {
			if arr, ok := op.Operands[0].([]interface{}); ok {
				var text string
				for _, item := range arr {
					if s, ok := item.(string); ok {
						text += s
					}
				}
				if i.OnText != nil && text != "" {
					i.OnText(text, state)
				}
			}
		}
	case "'":
		// Move to next line and show text
		state.TextState.LineMatrix = Translate(0, -state.TextState.Leading).Multiply(state.TextState.LineMatrix)
		state.TextState.TextMatrix = state.TextState.LineMatrix
		if len(op.Operands) >= 1 && i.OnText != nil {
			i.OnText(toString(op.Operands[0]), state)
		}
	case "\"":
		// Set word/char spacing, move to next line, show text
		if len(op.Operands) >= 3 {
			state.TextState.WordSpace = toFloat(op.Operands[0])
			state.TextState.CharSpace = toFloat(op.Operands[1])
			state.TextState.LineMatrix = Translate(0, -state.TextState.Leading).Multiply(state.TextState.LineMatrix)
			state.TextState.TextMatrix = state.TextState.LineMatrix
			if i.OnText != nil {
				i.OnText(toString(op.Operands[2]), state)
			}
		}
		
	// XObject operators
	case "Do":
		if len(op.Operands) >= 1 {
			if i.OnImage != nil {
				i.OnImage(toString(op.Operands[0]), state)
			}
		}
	}
	
	return nil
}

// parseColor creates a Color from operands based on the color space.
func (i *Interpreter) parseColor(space ColorSpace, operands []interface{}) Color {
	switch space {
	case ColorSpaceDeviceGray:
		if len(operands) >= 1 {
			return NewGray(toFloat(operands[0]))
		}
	case ColorSpaceDeviceRGB:
		if len(operands) >= 3 {
			return NewRGB(
				toFloat(operands[0]),
				toFloat(operands[1]),
				toFloat(operands[2]),
			)
		}
	case ColorSpaceCMYK:
		if len(operands) >= 4 {
			return NewCMYK(
				toFloat(operands[0]),
				toFloat(operands[1]),
				toFloat(operands[2]),
				toFloat(operands[3]),
			)
		}
	}
	return Black()
}

// applyExtGState applies an extended graphics state dictionary.
func (i *Interpreter) applyExtGState(name string) {
	// This would look up the ExtGState in Resources and apply it
	// For now, just a placeholder
	_ = name
}

// Helper functions for type conversion
func toFloat(v interface{}) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	}
	return 0
}

func toInt(v interface{}) int {
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case string:
		i, _ := strconv.Atoi(x)
		return i
	}
	return 0
}

func toString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case []byte:
		return string(x)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ParseContentStream parses a PDF content stream into operators.
func ParseContentStream(data []byte) ([]Operator, error) {
	var ops []Operator
	var operands []interface{}
	
	tokens := tokenize(string(data))
	
	for _, tok := range tokens {
		if isOperator(tok) {
			ops = append(ops, Operator{
				Name:     tok,
				Operands: operands,
			})
			operands = nil
		} else {
			operands = append(operands, parseOperand(tok))
		}
	}
	
	return ops, nil
}

// tokenize splits content stream into tokens.
func tokenize(s string) []string {
	var tokens []string
	var current strings.Builder
	inString := false
	parenDepth := 0
	inHex := false
	
	for i := 0; i < len(s); i++ {
		c := s[i]
		
		if inString {
			current.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				current.WriteByte(s[i])
				continue
			}
			if c == '(' {
				parenDepth++
			} else if c == ')' {
				parenDepth--
				if parenDepth == 0 {
					tokens = append(tokens, current.String())
					current.Reset()
					inString = false
				}
			}
			continue
		}
		
		if inHex {
			current.WriteByte(c)
			if c == '>' {
				tokens = append(tokens, current.String())
				current.Reset()
				inHex = false
			}
			continue
		}
		
		switch c {
		case '(':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			current.WriteByte(c)
			inString = true
			parenDepth = 1
		case '<':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			current.WriteByte(c)
			inHex = true
		case '[':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, "[")
		case ']':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, "]")
		case ' ', '\t', '\r', '\n':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case '/':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			// Read name
			current.WriteByte(c)
			for i+1 < len(s) && !isDelimiter(s[i+1]) && !isSpace(s[i+1]) {
				i++
				current.WriteByte(s[i])
			}
			tokens = append(tokens, current.String())
			current.Reset()
		case '%':
			// Skip comment
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			for i < len(s) && s[i] != '\n' && s[i] != '\r' {
				i++
			}
		default:
			current.WriteByte(c)
		}
	}
	
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	
	return tokens
}

func isDelimiter(c byte) bool {
	return c == '(' || c == ')' || c == '<' || c == '>' ||
		c == '[' || c == ']' || c == '/' || c == '%'
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

// isOperator returns true if the token is a PDF operator.
func isOperator(tok string) bool {
	// Numbers and names are not operators
	if len(tok) == 0 {
		return false
	}
	if tok[0] == '/' || tok[0] == '(' || tok[0] == '<' || tok[0] == '[' || tok[0] == ']' {
		return false
	}
	// Check if it's a number
	if (tok[0] >= '0' && tok[0] <= '9') || tok[0] == '-' || tok[0] == '+' || tok[0] == '.' {
		return false
	}
	// true, false, null are operands
	if tok == "true" || tok == "false" || tok == "null" {
		return false
	}
	return true
}

// parseOperand converts a token to an operand value.
func parseOperand(tok string) interface{} {
	if len(tok) == 0 {
		return nil
	}
	
	// String literal
	if tok[0] == '(' && tok[len(tok)-1] == ')' {
		return decodeString(tok[1 : len(tok)-1])
	}
	
	// Hex string
	if tok[0] == '<' && tok[len(tok)-1] == '>' {
		return decodeHexString(tok[1 : len(tok)-1])
	}
	
	// Name
	if tok[0] == '/' {
		return tok[1:]
	}
	
	// Boolean
	if tok == "true" {
		return true
	}
	if tok == "false" {
		return false
	}
	
	// Null
	if tok == "null" {
		return nil
	}
	
	// Number
	if f, err := strconv.ParseFloat(tok, 64); err == nil {
		return f
	}
	
	return tok
}

// decodeString decodes escape sequences in a PDF string.
func decodeString(s string) string {
	var result strings.Builder
	
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'n':
				result.WriteByte('\n')
			case 'r':
				result.WriteByte('\r')
			case 't':
				result.WriteByte('\t')
			case 'b':
				result.WriteByte('\b')
			case 'f':
				result.WriteByte('\f')
			case '(':
				result.WriteByte('(')
			case ')':
				result.WriteByte(')')
			case '\\':
				result.WriteByte('\\')
			default:
				// Octal?
				if s[i] >= '0' && s[i] <= '7' {
					oct := string(s[i])
					for j := 0; j < 2 && i+1 < len(s) && s[i+1] >= '0' && s[i+1] <= '7'; j++ {
						i++
						oct += string(s[i])
					}
					if v, err := strconv.ParseUint(oct, 8, 8); err == nil {
						result.WriteByte(byte(v))
					}
				} else {
					result.WriteByte(s[i])
				}
			}
		} else {
			result.WriteByte(s[i])
		}
	}
	
	return result.String()
}

// decodeHexString decodes a hex-encoded PDF string.
func decodeHexString(s string) string {
	var result strings.Builder
	var hex byte
	var hasNibble bool
	
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		
		var nibble byte
		if c >= '0' && c <= '9' {
			nibble = c - '0'
		} else if c >= 'A' && c <= 'F' {
			nibble = c - 'A' + 10
		} else if c >= 'a' && c <= 'f' {
			nibble = c - 'a' + 10
		} else {
			continue
		}
		
		if hasNibble {
			result.WriteByte(hex<<4 | nibble)
			hasNibble = false
		} else {
			hex = nibble
			hasNibble = true
		}
	}
	
	if hasNibble {
		result.WriteByte(hex << 4)
	}
	
	return result.String()
}
