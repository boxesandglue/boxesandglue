package pdf

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

// The PNG decoder is copied from https://github.com/signintech/gopdf and
// adapted to the needs for boxesandglue. gopdf is covered by this license:

// The MIT License (MIT)
//
// Copyright (c) 2015 signintech
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

func readUInt(f io.Reader) (uint, error) {
	buff, err := readBytes(f, 4)
	if err != nil {
		return 0, err
	}
	n := binary.BigEndian.Uint32(buff)
	return uint(n), nil
}

func readInt(f io.Reader) (int, error) {
	u, err := readUInt(f)
	if err != nil {
		return 0, err
	}
	var v int
	if u >= 0x8000 {
		v = int(u) - 65536
	} else {
		v = int(u)
	}
	return v, nil
}

func readBytes(f io.Reader, len int) ([]byte, error) {
	b := make([]byte, len)
	_, err := f.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func compress(data []byte) ([]byte, error) {
	var buff bytes.Buffer
	zwr := zlib.NewWriter(&buff)
	var err error
	_, err = zwr.Write(data)
	if err != nil {
		return nil, err
	}
	zwr.Close()
	return buff.Bytes(), nil
}

// from gopdf
func (imgf *Imagefile) parsePNG() error {
	imgf.r.Seek(0, io.SeekStart)
	b, err := readBytes(imgf.r, 8)
	if err != nil {
		return err
	}
	if !bytes.Equal(b, []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		return errors.New("Not a PNG file")
	}

	imgf.r.Seek(4, io.SeekCurrent) //skip header chunk
	b, err = readBytes(imgf.r, 4)
	if err != nil {
		return err
	}
	// IHDR
	if !bytes.Equal(b, []byte{0x49, 0x48, 0x44, 0x52}) {
		return errors.New("Incorrect PNG file")
	}

	w, err := readInt(imgf.r)
	if err != nil {
		return err
	}
	h, err := readInt(imgf.r)
	if err != nil {
		return err
	}
	imgf.W = w
	imgf.H = h

	bpc, err := readBytes(imgf.r, 1)
	if err != nil {
		return err
	}

	if bpc[0] > 8 {
		return errors.New("16-bit depth not supported")
	}

	ct, err := readBytes(imgf.r, 1)
	if err != nil {
		return err
	}

	var colspace string
	switch ct[0] {
	case 0, 4:
		colspace = "DeviceGray"
	case 2, 6:
		colspace = "DeviceRGB"
	case 3:
		colspace = "Indexed"
	default:
		return errors.New("Unknown color type")
	}

	compressionMethod, err := readBytes(imgf.r, 1)
	if err != nil {
		return err
	}
	if compressionMethod[0] != 0 {
		return errors.New("Unknown compression method")
	}

	filterMethod, err := readBytes(imgf.r, 1)
	if err != nil {
		return err
	}
	if filterMethod[0] != 0 {
		return errors.New("Unknown filter method")
	}

	interlacing, err := readBytes(imgf.r, 1)
	if err != nil {
		return err
	}
	if interlacing[0] != 0 {
		return errors.New("Interlacing not supported")
	}

	_, err = imgf.r.Seek(4, io.SeekCurrent)
	if err != nil {
		return err
	}

	var pal []byte
	var trns []byte
	var data []byte
	for {
		un, err := readUInt(imgf.r)
		if err != nil {
			return err
		}
		n := int(un)
		typ, err := readBytes(imgf.r, 4)
		//fmt.Printf(">>>>%+v-%s-%d\n", typ, string(typ), n)
		if err != nil {
			return err
		}

		if string(typ) == "PLTE" {
			if pal, err = readBytes(imgf.r, n); err != nil {
				return err
			}
			if _, err = imgf.r.Seek(int64(4), io.SeekCurrent); err != nil {
				return err
			}
		} else if string(typ) == "tRNS" { // Transparency
			var t []byte
			t, err = readBytes(imgf.r, n)
			if err != nil {
				return err
			}

			if ct[0] == 0 {
				trns = []byte{(t[1])}
			} else if ct[0] == 2 {
				trns = []byte{t[1], t[3], t[5]}
			} else {
				pos := strings.Index(string(t), "\x00")
				if pos >= 0 {
					trns = []byte{byte(pos)}
				}
			}

			_, err = imgf.r.Seek(int64(4), io.SeekCurrent)
			if err != nil {
				return err
			}

		} else if string(typ) == "IDAT" { // Image data
			var d []byte
			d, err = readBytes(imgf.r, n)
			if err != nil {
				return err
			}
			data = append(data, d...)
			_, err = imgf.r.Seek(int64(4), io.SeekCurrent)
			if err != nil {
				return err
			}
		} else if string(typ) == "IEND" { // Image trailer
			break
		} else {
			_, err = imgf.r.Seek(int64(n+4), io.SeekCurrent)
			if err != nil {
				return err
			}
		}

		if n <= 0 {
			break
		}
	} //end for

	imgf.trns = trns
	imgf.pal = pal

	if colspace == "Indexed" && strings.TrimSpace(string(pal)) == "" {
		return errors.New("Missing palette")
	}

	imgf.colorspace = colspace
	imgf.bitsPerComponent = fmt.Sprintf("%d", int(bpc[0]))
	imgf.filter = "FlateDecode"

	colors := 1
	if colspace == "DeviceRGB" {
		colors = 3
	}
	imgf.decodeParms = fmt.Sprintf("/Predictor 15 /Colors  %d /BitsPerComponent %s /Columns %d", colors, imgf.bitsPerComponent, w)

	if ct[0] < 4 {
		imgf.data = data
		return nil
	}
	zipReader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer zipReader.Close()
	afterZipData, err := ioutil.ReadAll(zipReader)
	if err != nil {
		return err
	}

	var color []byte
	var alpha []byte
	if ct[0] == 4 {
		// Gray image
		length := 2 * w
		i := 0
		for i < h {
			pos := (1 + length) * i
			color = append(color, afterZipData[pos])
			alpha = append(alpha, afterZipData[pos])
			line := afterZipData[pos+1 : pos+length+1]
			j := 0
			max := len(line)
			for j < max {
				color = append(color, line[j])
				j++
				alpha = append(alpha, line[j])
				j++
			}
			i++
		}
	} else {
		// RGB image
		length := 4 * w
		i := 0
		for i < h {
			pos := (1 + length) * i
			color = append(color, afterZipData[pos])
			alpha = append(alpha, afterZipData[pos])
			line := afterZipData[pos+1 : pos+length+1]
			j := 0
			max := len(line)
			for j < max {
				color = append(color, line[j:j+3]...)
				alpha = append(alpha, line[j+3])
				j = j + 4
			}

			i++
		}
		imgf.smask, err = compress(alpha)
		if err != nil {
			return err
		}

		imgf.data, err = compress(color)
		if err != nil {
			return err
		}
	}
	return nil
}
