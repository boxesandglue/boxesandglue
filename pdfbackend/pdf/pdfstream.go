package pdf

// A Stream holds any kind of data.
type Stream struct {
	data []byte
	dict Dict
}

// NewStream creates a stream of data
func NewStream(data []byte) *Stream {
	s := Stream{}
	s.data = data
	s.dict = make(Dict)
	return &s
}
