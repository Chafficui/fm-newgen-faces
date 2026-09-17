//go:build ignore

// Command gen_icon generates assets/icon.png: a 256x256 dark-green "pitch"
// tile holding a white rounded card with a simple avatar/face silhouette.
// It is a one-off generator, not part of the build.
//
// Run with: go run assets/gen/gen_icon.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

const size = 256

func main() {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	pitch := color.RGBA{0x0f, 0x4d, 0x2c, 0xff}       // dark pitch green
	pitchStripe := color.RGBA{0x0c, 0x42, 0x25, 0xff} // slightly darker stripe
	white := color.RGBA{0xff, 0xff, 0xff, 0xff}
	silhouette := color.RGBA{0x0f, 0x4d, 0x2c, 0xff} // same as pitch: cut-out look

	const stripeWidth = size / 8
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			c := pitch
			if (x/stripeWidth)%2 == 1 {
				c = pitchStripe
			}
			img.Set(x, y, c)
		}
	}

	const (
		margin = 30
		radius = 30
	)
	minX, minY := margin, margin
	maxX, maxY := size-margin, size-margin
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			if inRoundedRect(x, y, minX, minY, maxX, maxY, radius) {
				img.Set(x, y, white)
			}
		}
	}

	// Face silhouette: a head (circle) over shoulders (ellipse), both cut to
	// the card's rounded bounds.
	cx := size / 2
	headCY, headR := 106.0, 34.0
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx, dy := float64(x-cx), float64(y)-headCY
			if dx*dx+dy*dy <= headR*headR {
				img.Set(x, y, silhouette)
			}
		}
	}

	shoulderCY, shoulderRX, shoulderRY := 210.0, 58.0, 46.0
	for y := minY; y < maxY; y++ {
		for x := minX; x < maxX; x++ {
			dx := float64(x-cx) / shoulderRX
			dy := (float64(y) - shoulderCY) / shoulderRY
			if dx*dx+dy*dy <= 1 && inRoundedRect(x, y, minX, minY, maxX, maxY, radius) {
				img.Set(x, y, silhouette)
			}
		}
	}

	out, err := os.Create("assets/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer out.Close()

	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(out, img); err != nil {
		log.Fatal(err)
	}
}

// inRoundedRect reports whether (x, y) is inside the rectangle
// [minX,maxX)x[minY,maxY) with its four corners rounded to radius r.
func inRoundedRect(x, y, minX, minY, maxX, maxY, r int) bool {
	if x < minX || x >= maxX || y < minY || y >= maxY {
		return false
	}
	switch {
	case x < minX+r && y < minY+r:
		return withinCorner(x, y, minX+r, minY+r, r)
	case x >= maxX-r && y < minY+r:
		return withinCorner(x, y, maxX-r-1, minY+r, r)
	case x < minX+r && y >= maxY-r:
		return withinCorner(x, y, minX+r, maxY-r-1, r)
	case x >= maxX-r && y >= maxY-r:
		return withinCorner(x, y, maxX-r-1, maxY-r-1, r)
	default:
		return true
	}
}

func withinCorner(x, y, cx, cy, r int) bool {
	dx, dy := float64(x-cx), float64(y-cy)
	return dx*dx+dy*dy <= math.Pow(float64(r), 2)
}
