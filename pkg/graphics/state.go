package graphics

// State represents the complete graphics state.
// PDF maintains a stack of these states using q/Q operators.
type State struct {
	// Current Transformation Matrix
	CTM Matrix
	
	// Clipping path (nil = no clipping)
	ClipPath *Path
	
	// Color state
	StrokeColor    Color
	FillColor      Color
	StrokeColorSpace ColorSpace
	FillColorSpace   ColorSpace
	
	// Line drawing parameters
	LineWidth   float64
	LineCap     LineCap
	LineJoin    LineJoin
	MiterLimit  float64
	DashPattern []float64
	DashPhase   float64
	
	// Text state
	TextState TextState
	
	// Transparency
	StrokeAlpha float64
	FillAlpha   float64
	BlendMode   BlendMode
	
	// Rendering intent
	RenderingIntent string
	
	// Flatness
	Flatness float64
	
	// Smoothness
	Smoothness float64
}

// TextState contains text-specific state.
type TextState struct {
	// Character spacing (Tc)
	CharSpace float64
	
	// Word spacing (Tw)
	WordSpace float64
	
	// Horizontal scaling (Th) - percentage
	HScale float64
	
	// Leading (Tl) - line spacing
	Leading float64
	
	// Font name and size
	FontName string
	FontSize float64
	
	// Text rendering mode (Tr)
	RenderMode TextRenderMode
	
	// Text rise (Ts)
	Rise float64
	
	// Text matrix (Tm)
	TextMatrix Matrix
	
	// Text line matrix (for Td, TD, T*, ', ")
	LineMatrix Matrix
}

// TextRenderMode defines how text is rendered.
type TextRenderMode int

const (
	TextRenderFill          TextRenderMode = 0
	TextRenderStroke        TextRenderMode = 1
	TextRenderFillStroke    TextRenderMode = 2
	TextRenderInvisible     TextRenderMode = 3
	TextRenderFillClip      TextRenderMode = 4
	TextRenderStrokeClip    TextRenderMode = 5
	TextRenderFillStrokeClip TextRenderMode = 6
	TextRenderClip          TextRenderMode = 7
)

// NewState creates a new graphics state with default values.
func NewState() *State {
	return &State{
		CTM: Identity(),
		
		StrokeColor:      Black(),
		FillColor:        Black(),
		StrokeColorSpace: ColorSpaceDeviceGray,
		FillColorSpace:   ColorSpaceDeviceGray,
		
		LineWidth:  1.0,
		LineCap:    LineCapButt,
		LineJoin:   LineJoinMiter,
		MiterLimit: 10.0,
		
		StrokeAlpha: 1.0,
		FillAlpha:   1.0,
		BlendMode:   BlendNormal,
		
		RenderingIntent: "RelativeColorimetric",
		Flatness:        1.0,
		Smoothness:      0.0,
		
		TextState: TextState{
			HScale:     100,
			RenderMode: TextRenderFill,
			TextMatrix: Identity(),
			LineMatrix: Identity(),
		},
	}
}

// Clone creates a deep copy of the state.
func (s *State) Clone() *State {
	clone := *s
	
	// Deep copy the dash pattern
	if s.DashPattern != nil {
		clone.DashPattern = make([]float64, len(s.DashPattern))
		copy(clone.DashPattern, s.DashPattern)
	}
	
	// Clone clip path if present
	if s.ClipPath != nil {
		clone.ClipPath = s.ClipPath.Clone()
	}
	
	return &clone
}

// StateStack manages a stack of graphics states.
type StateStack struct {
	states []*State
}

// NewStateStack creates a new state stack with an initial state.
func NewStateStack() *StateStack {
	return &StateStack{
		states: []*State{NewState()},
	}
}

// Current returns the current (topmost) state.
func (s *StateStack) Current() *State {
	if len(s.states) == 0 {
		s.states = append(s.states, NewState())
	}
	return s.states[len(s.states)-1]
}

// Push saves the current state (PDF 'q' operator).
func (s *StateStack) Push() {
	current := s.Current()
	s.states = append(s.states, current.Clone())
}

// Pop restores the previous state (PDF 'Q' operator).
func (s *StateStack) Pop() {
	if len(s.states) > 1 {
		s.states = s.states[:len(s.states)-1]
	}
}

// Depth returns the current stack depth.
func (s *StateStack) Depth() int {
	return len(s.states)
}
