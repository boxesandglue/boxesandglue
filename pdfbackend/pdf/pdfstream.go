package pdf

// A Stream holds any kind of data.
type Stream struct {
	data     []byte
	dict     Dict
	compress bool
}

// NewStream creates a stream of data
func NewStream(data []byte) *Stream {
	s := Stream{}
	s.data = data
	s.dict = make(Dict)
	return &s
}

// SetCompression turns on stream compression
func (s *Stream) SetCompression(compresslevel uint) {
	s.compress = compresslevel > 0
}
