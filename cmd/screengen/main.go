// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the Free
// Software Foundation, either version 3 of the License, or (at your option)
// any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU General
// Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program.  If not, see <http://www.gnu.org/licenses/>.

// A tool that makes screenlists from video files with the help of ImageMagick's convert.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"

	"github.com/opennota/screengen"
)

const (
	thWidth   = 256
	thSpacing = 16
)

var (
	n                = flag.Int("n", 27, "Number of thumbnails")
	thumbnailsPerRow = flag.Int("thumbnails-per-row", 3, "Thumbnails per row")
	output           = flag.String("o", "output.jpg", "Output file")
	quality          = flag.Int("quality", 85, "Output image quality")
)

func ms2String(ms int64) string {
	s := ms / 1000
	h := s / 60 / 60
	s -= h * 60 * 60
	m := s / 60
	s -= m * 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

type Image struct {
	time     int64
	filename string
}

func writePng(img image.Image) (string, error) {
	f, err := ioutil.TempFile("", "screengen")
	if err != nil {
		return "", err
	}

	err = png.Encode(f, img)
	if err != nil {
		f.Close()
		return "", err
	}

	err = f.Close()
	if err != nil {
		return "", err
	}

	return f.Name(), nil
}

func divRoundUp(a, b int) int {
	c := a / b
	if a%b > 0 {
		c++
	}
	return c
}

func fileSize(name string) string {
	fi, err := os.Stat(name)
	if err != nil {
		log.Fatal("can't get file size:", err)
	}
	return fmt.Sprintf("%d Mb", fi.Size()/1024/1024)
}

func makeScreenList(g *screengen.Generator) error {
	thHeight := int(float64(g.Height) * thWidth / float64(g.Width))
	images := make([]Image, 0, *n)
	inc := g.Duration / int64(*n)
	var d int64
	for i := 0; i < *n; i++ {
		img, err := g.ImageWxH(d, thWidth, thHeight)
		if err != nil {
			return fmt.Errorf("can't extract image: %v", err)
		}

		fn, err := writePng(img)
		if err != nil {
			return fmt.Errorf("can't write thumbnail: %v", err)
		}
		defer os.Remove(fn)

		images = append(images, Image{
			time:     d,
			filename: fn,
		})

		d += inc
	}

	numRows := divRoundUp(len(images), *thumbnailsPerRow)
	w := *thumbnailsPerRow*thWidth + (*thumbnailsPerRow+1)*thSpacing
	h := numRows*thHeight + (numRows+1)*thSpacing
	const xoffset = 80
	const lineHeight = 16
	args := []string{
		"(",
		"-size", fmt.Sprintf("%dx%d", w, 128),
		"xc:white",
		"-fill", "black",

		"-font", "LiberationSansB",
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing, thSpacing*2, "Filename:"),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing, thSpacing*2+lineHeight, "Size:"),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing, thSpacing*2+lineHeight*2, "Duration:"),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing, thSpacing*2+lineHeight*3, "Resolution:"),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing, thSpacing*2+lineHeight*4, "Video:"),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing, thSpacing*2+lineHeight*5, "Audio:"),

		"-font", "LiberationSans",
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing+xoffset, thSpacing*2, g.Filename),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing+xoffset, thSpacing*2+lineHeight, fileSize(g.Filename)),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing+xoffset, thSpacing*2+lineHeight*2, ms2String(g.Duration)),
		"-draw", fmt.Sprintf("text %d,%d '%dx%d'", thSpacing+xoffset, thSpacing*2+lineHeight*3, g.Width, g.Height),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing+xoffset, thSpacing*2+lineHeight*4, g.VideoCodecLongName),
		"-draw", fmt.Sprintf("text %d,%d '%s'", thSpacing+xoffset, thSpacing*2+lineHeight*5, g.AudioCodecLongName),
		")",

		"(",
		"-size", fmt.Sprintf("%dx%d", w, h),
		"xc:white",
		"-gravity", "northwest",
		"-font", "LiberationSans",
	}

	x := 0
	y := 0
	for _, img := range images {
		args = append(args,
			img.filename,
			"-geometry", fmt.Sprintf("+%d+%d", x*(thWidth+thSpacing)+thSpacing, y*(thHeight+thSpacing)+thSpacing),
			"-composite",
			"-fill", "black",
			"-draw", fmt.Sprintf("text %d,%d '%s'", x*(thWidth+thSpacing)+thSpacing+5, y*(thHeight+thSpacing)+thSpacing+5, ms2String(img.time)),
			"-fill", "white",
			"-draw", fmt.Sprintf("text %d,%d '%s'", x*(thWidth+thSpacing)+thSpacing+6, y*(thHeight+thSpacing)+thSpacing+6, ms2String(img.time)),
		)

		x++
		if x == *thumbnailsPerRow {
			x = 0
			y++
		}
	}

	args = append(args, ")", "-append", "-quality", strconv.Itoa(*quality), *output)

	return exec.Command("convert", args...).Run()
}

func main() {
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Println("Usage: screengen [options] videofile")
		fmt.Println("Options:")
		flag.PrintDefaults()
		return
	}

	g, err := screengen.NewGenerator(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	err = makeScreenList(g)
	if err != nil {
		log.Fatal(err)
	}
}
