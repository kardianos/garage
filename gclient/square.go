// Copyright 2015 Daniel Theophanes.
// Use of this source code is governed by a zlib-style
// license that can be found in the LICENSE file.package service

package main

import (
	"image"

	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/exp/font"
	"golang.org/x/mobile/exp/gl/glutil"
	"golang.org/x/mobile/geom"
	"golang.org/x/mobile/gl"

	"github.com/golang/freetype"
	"golang.org/x/image/colornames"
	"golang.org/x/image/draw"
	ifont "golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type Square struct {
	percent float32

	images *glutil.Images
	img1   *glutil.Image

	ftctx *freetype.Context
}

func NewSquare(glctx gl.Context, inc, x, y float32) (*Square, error) {
	images := glutil.NewImages(glctx)
	img1 := images.NewImage(1600, 200)

	ftctx := freetype.NewContext()
	ftFont, err := freetype.ParseFont(font.Default())
	if err != nil {
		return nil, err
	}
	ftctx.SetFont(ftFont)

	ftctx.SetDPI(312)
	ftctx.SetSrc(image.NewUniform(colornames.Map["black"]))
	ftctx.SetDst(img1.RGBA)
	ftctx.SetClip(img1.RGBA.Bounds())
	ftctx.SetHinting(ifont.HintingFull)

	sq := &Square{
		images: images,
		img1:   img1,
		ftctx:  ftctx,
	}
	return sq, nil
}

func (sq *Square) Close(glctx gl.Context) {
	if sq == nil {
		return
	}

	sq.img1.Release()
	sq.images.Release()
}

func (sq *Square) Draw(glctx gl.Context, sz size.Event, major, minor string) {
	if sq == nil {
		return
	}
	draw.Copy(sq.img1.RGBA, image.ZP, image.NewUniform(colornames.Map["deepskyblue"]), sq.img1.RGBA.Bounds(), draw.Src, nil)

	ftctx := sq.ftctx

	ftctx.SetFontSize(12)
	ftctx.DrawString(major, fixed.Point26_6{X: 100, Y: 7000})

	ftctx.SetFontSize(6)
	ftctx.DrawString(minor, fixed.Point26_6{X: 100, Y: 11000})

	b := sq.img1.RGBA.Bounds()

	sq.img1.Upload()
	sq.img1.Draw(sz, geom.Point{X: 0, Y: 0}, geom.Point{X: geom.Pt(b.Max.X) / 4, Y: 0}, geom.Point{X: 0, Y: geom.Pt(b.Max.Y) / 4}, sq.img1.RGBA.Bounds())
}
func (sq *Square) SetLocation(x, y float32) {
	// sq.x, sq.y = x, y
}
