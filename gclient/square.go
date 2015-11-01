// Copyright 2015 Daniel Theophanes.
// Use of this source code is governed by a zlib-style
// license that can be found in the LICENSE file.package service

package main

import (
	"encoding/binary"
	"fmt"
	"image"
	"time"

	"golang.org/x/mobile/event/size"
	"golang.org/x/mobile/exp/f32"
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
	program  gl.Program
	position gl.Attrib
	offset   gl.Uniform
	color    gl.Uniform
	buf      gl.Buffer

	green float32
	down  bool
	x, y  float32
	last  time.Time
	inc   float32

	images *glutil.Images
	img1   *glutil.Image

	ftctx *freetype.Context
}

func NewSquare(glctx gl.Context, inc, x, y float32) (*Square, error) {
	program, err := glutil.CreateProgram(glctx, vertexShader, fragmentShader)
	if err != nil {
		return nil, fmt.Errorf("error creating GL program: %v", err)
	}

	var (
		sqX          float32 = x
		sqY          float32 = y
		triangleData         = f32.Bytes(binary.LittleEndian,
			0.0, sqY, 0.0, // top left
			0.0, 0.0, 0.0, // bottom left
			sqX, 0.0, 0.0, // bottom right

			sqX, sqY, 0.0, // top right
			sqX, 0.0, 0.0, // bottom right
			0.0, sqY, 0.0, // bottom left
		)
	)

	images := glutil.NewImages(glctx)

	ftctx := freetype.NewContext()
	ftFont, err := freetype.ParseFont(font.Default())
	if err != nil {
		panic(err)
	}
	ftctx.SetFont(ftFont)

	sq := &Square{
		images:  images,
		img1:    images.NewImage(400, 200),
		ftctx:   ftctx,
		program: program,
		last:    time.Now(),
		inc:     inc,
	}
	sq.buf = glctx.CreateBuffer()
	glctx.BindBuffer(gl.ARRAY_BUFFER, sq.buf)
	glctx.BufferData(gl.ARRAY_BUFFER, triangleData, gl.STATIC_DRAW)

	sq.position = glctx.GetAttribLocation(program, "position")
	sq.color = glctx.GetUniformLocation(program, "color")
	sq.offset = glctx.GetUniformLocation(program, "offset")
	return sq, nil
}

func (sq *Square) Close(glctx gl.Context) {
	if sq == nil {
		return
	}
	glctx.DeleteProgram(sq.program)
	glctx.DeleteBuffer(sq.buf)

	sq.img1.Release()
	sq.images.Release()
}

func (sq *Square) Draw(glctx gl.Context, sz size.Event) {
	if sq == nil {
		return
	}
	const (
		coordsPerVertex = 3
		vertexCount     = 6
	)
	glctx.UseProgram(sq.program)

	now := time.Now()
	var inc = float32(0.01) * float32(now.Sub(sq.last).Seconds()*1000) / 15
	sq.last = now
	if sq.down {
		sq.green -= inc
	} else {
		sq.green += inc
	}
	var (
		limHigh float32 = 0.9
		limLow  float32 = 0.2
	)
	if (!sq.down && sq.green >= limHigh) || (sq.down && sq.green <= limLow) {
		sq.down = !sq.down
	}
	if sq.green < limLow {
		sq.green = limLow
	}
	if sq.green > limHigh {
		sq.green = limHigh
	}
	glctx.Uniform4f(sq.color, 0, sq.green, 0, 1)

	glctx.Uniform2f(sq.offset, sq.x/float32(sz.WidthPx), sq.y/float32(sz.HeightPx))

	glctx.BindBuffer(gl.ARRAY_BUFFER, sq.buf)
	glctx.EnableVertexAttribArray(sq.position)
	glctx.VertexAttribPointer(sq.position, coordsPerVertex, gl.FLOAT, false, 0, 0)
	glctx.DrawArrays(gl.TRIANGLES, 0, vertexCount)
	glctx.DisableVertexAttribArray(sq.position)

	draw.Copy(sq.img1.RGBA, image.ZP, image.NewUniform(colornames.Map["deepskyblue"]), sq.img1.RGBA.Bounds(), draw.Src, nil)

	ftctx := sq.ftctx
	ftctx.SetFontSize(12)
	ftctx.SetDPI(312)
	ftctx.SetSrc(image.NewUniform(colornames.Map["black"]))
	ftctx.SetDst(sq.img1.RGBA)
	ftctx.SetClip(sq.img1.RGBA.Bounds())
	ftctx.SetHinting(ifont.HintingFull)
	ftctx.DrawString("Hello Mobile", fixed.Point26_6{X: 100, Y: 10000})

	sq.img1.Upload()
	sq.img1.Draw(sz, geom.Point{X: 0, Y: 0}, geom.Point{X: 50, Y: 0}, geom.Point{X: 0, Y: 25}, sq.img1.RGBA.Bounds())
}
func (sq *Square) SetLocation(x, y float32) {
	sq.x, sq.y = x, y
}

const vertexShader = `#version 100
uniform vec2 offset;

attribute vec4 position;
void main() {
	// offset comes in with x/y values between 0 and 1.
	// position bounds are -1 to 1.
	vec4 offset4 = vec4(2.0*offset.x-1.0, 1.0-2.0*offset.y, 0, 0);
	gl_Position = position + offset4;
}`

const fragmentShader = `#version 100
precision mediump float;
uniform vec4 color;
void main() {
	gl_FragColor = color;
}`
