package type1

// The typcial PFB font is structured like this:
// 128 01  <four bytes that encode the length of first segment>
// %!PS-AdobeFont-1.0
// % ...
// currentfile eexec
// 128 02 <four bytes that encode the length of the secod segment>
// binary data
// 128 01  <four bytes that encode the length of third segment>
// 0000000000 512 zeros  cleartomark{restore}if
// 128 03 (EOF)

import (
	"io"
	"io/ioutil"
)

// ParsePFB reads the contents of the file and store the result in the segments.
func (t *Type1) ParsePFB(r io.Reader) error {
	pfb, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}

	offset := uint32(0)

done_segments:
	for {
		if pfb[offset] == 128 {
			switch pfb[offset+1] {
			case 1, 2:
				// ascii, binary
				length := uint32(pfb[offset+2]) + uint32(pfb[offset+3])<<8 + uint32(pfb[offset+4])<<16 + uint32(pfb[offset+5])<<24
				t.Segments = append(t.Segments, pfb[offset+6:offset+length+6])
				offset = offset + 6 + length
			case 3:
				// EOF marker
				break done_segments
			}
		}
	}
	return nil
}
