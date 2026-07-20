// Command genicons renders the TitipDong PWA icons (192 & 512 px PNGs).
// Throwaway tool: run with `go run ./cmd/genicons` and move the output into web/static.
package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"strconv"
)

func main() {
	sky := color.RGBA{R: 14, G: 165, B: 233, A: 255} // sky-500
	white := color.RGBA{R: 255, G: 255, B: 255, A: 255}
	for _, s := range []int{192, 512} {
		img := image.NewRGBA(image.Rect(0, 0, s, s))
		for y := 0; y < s; y++ {
			for x := 0; x < s; x++ {
				img.Set(x, y, sky)
			}
		}
		// Center white block as a minimal "bag" glyph.
		m := s / 4
		for y := m; y < s-m; y++ {
			for x := m; x < s-m; x++ {
				img.Set(x, y, white)
			}
		}
		name := "icon-" + strconv.Itoa(s) + ".png"
		f, err := os.Create(name)
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, img); err != nil {
			panic(err)
		}
		f.Close()
		println("wrote", name)
	}
}
