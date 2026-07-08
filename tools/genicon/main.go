// Command genicon renders build/appicon.png (1024x1024) for the app bundle.
// Run from the repo root: go run ./tools/genicon
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const dim = 1024

// smooth maps a signed distance v (positive = inside) to 0..1 coverage over an
// anti-aliasing band of width w.
func smooth(v, w float64) float64 {
	t := v/w + 0.5
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

// rectAlpha is the coverage of the rounded-square background:
// margin 64 px on every side, corner radius 180 px.
func rectAlpha(x, y float64) float64 {
	const m, r = 64.0, 180.0
	lo, hi := m+r, float64(dim)-m-r
	cx := math.Min(math.Max(x, lo), hi)
	cy := math.Min(math.Max(y, lo), hi)
	d := math.Hypot(x-cx, y-cy)
	return smooth(r-d, 2.0)
}

// starAlpha is the coverage of a 4-point sparkle (astroid |x|^(2/3)+|y|^(2/3)=r^(2/3)).
func starAlpha(x, y, cx, cy, r float64) float64 {
	dx, dy := math.Abs(x-cx), math.Abs(y-cy)
	e := math.Pow(r, 2.0/3.0)
	v := math.Pow(dx, 2.0/3.0) + math.Pow(dy, 2.0/3.0)
	return smooth(e-v, 0.08*e)
}

func lerp(a, b, t float64) float64 { return a + (b-a)*t }

func main() {
	img := image.NewNRGBA(image.Rect(0, 0, dim, dim))
	top := [3]float64{0x4F, 0x46, 0xE5}    // indigo-600
	bottom := [3]float64{0x93, 0x33, 0xEA} // purple-600
	for py := 0; py < dim; py++ {
		for px := 0; px < dim; px++ {
			x, y := float64(px)+0.5, float64(py)+0.5
			a := rectAlpha(x, y)
			if a <= 0 {
				continue // fully transparent pixel
			}
			t := (x + y) / (2 * dim) // diagonal gradient
			cr := lerp(top[0], bottom[0], t)
			cg := lerp(top[1], bottom[1], t)
			cb := lerp(top[2], bottom[2], t)
			s := math.Max(starAlpha(x, y, 512, 540, 340),
				math.Max(starAlpha(x, y, 764, 268, 120), starAlpha(x, y, 292, 244, 70)))
			cr, cg, cb = lerp(cr, 255, s), lerp(cg, 255, s), lerp(cb, 255, s)
			img.SetNRGBA(px, py, color.NRGBA{
				R: uint8(cr + 0.5),
				G: uint8(cg + 0.5),
				B: uint8(cb + 0.5),
				A: uint8(a*255 + 0.5),
			})
		}
	}
	f, err := os.Create("build/appicon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
	log.Println("wrote build/appicon.png (1024x1024)")
}
