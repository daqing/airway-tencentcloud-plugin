// Package captcha issues and verifies image captchas for abuse-prone
// endpoints (sending SMS verification codes). Answers are 4 random digits
// rendered as a PNG with the standard library only; a captcha is bound to
// the requesting IP and expires after 10 minutes.
package captcha

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	mrand "math/rand"
	"strings"
	"time"

	"github.com/daqing/airway/lib/repo"
	"github.com/daqing/airway/lib/sql"
	"github.com/daqing/airway/lib/utils"

	"github.com/daqing/airway-tencentcloud-plugin/install/lib/models"
)

const (
	AnswerLength = 4
	TTL          = 10 * time.Minute

	// MaxPerIPPerHour caps how many captchas one IP may ask for per hour. The
	// endpoint that issues them is public and unauthenticated, and every call
	// costs a row and a PNG render, so an uncapped one is both a write
	// amplifier and a CPU sink.
	MaxPerIPPerHour = 30
)

// ErrRateLimited reports that the caller has requested too many captchas.
var ErrRateLimited = errors.New("too many captchas, please try again later")

// Issue creates a captcha for the given IP and returns its id and PNG image.
// It reports ErrRateLimited instead of issuing once the IP is over
// MaxPerIPPerHour, and does so before writing anything or rendering.
func Issue(ctx context.Context, ip string) (string, []byte, error) {
	limited, err := rateLimited(ip)
	if err != nil {
		return "", nil, err
	}
	if limited {
		return "", nil, ErrRateLimited
	}

	answer, err := randomDigits(AnswerLength)
	if err != nil {
		return "", nil, err
	}

	id := utils.RandomHex(16)
	now := time.Now()
	if _, err := repo.CreateFrom[models.Captcha](sql.H{
		"id":         id,
		"answer":     answer,
		"ip":         ip,
		"expires_at": now.Add(TTL),
		"created_at": now,
		"updated_at": now,
	}); err != nil {
		return "", nil, err
	}

	pngBytes, err := renderPNG(answer)
	if err != nil {
		return "", nil, err
	}

	return id, pngBytes, nil
}

// rateLimited reports whether ip has asked for MaxPerIPPerHour captchas or
// more in the last hour. Expired and consumed rows count too: they still
// represent a request that was served.
func rateLimited(ip string) (bool, error) {
	issued, err := repo.Join(models.Captcha{}).
		Where(sql.AllOf(
			sql.Eq("captchas.ip", ip),
			sql.Gte("captchas.created_at", time.Now().Add(-time.Hour)),
		)).
		Count()
	if err != nil {
		return false, err
	}

	return issued >= MaxPerIPPerHour, nil
}

// Verify consumes a captcha and reports whether it was valid: same IP, not
// consumed, not expired, and the answer matches. The first Verify call wins —
// it consumes the captcha whatever the answer, so a wrong guess burns it too.
//
// Consumption is a conditional UPDATE rather than a read-then-write, so two
// concurrent callers cannot both consume the same captcha.
func Verify(ctx context.Context, ip, id, answer string) bool {
	c, err := repo.FindOneBy[models.Captcha](sql.H{"id": id})
	if err != nil || c == nil {
		return false
	}

	now := time.Now()
	consumed, err := repo.UpdateAffected(repo.CurrentDB(),
		sql.UpdateAll(models.Captcha{}, sql.H{"consumed": true, "updated_at": now}).
			Where(sql.AllOf(sql.Eq("id", id), sql.Eq("consumed", false))))
	if err != nil || consumed == 0 {
		// Already consumed, or another caller got there first.
		return false
	}

	return c.IP == ip && !c.Expired() && answer == c.Answer
}

func randomDigits(n int) (string, error) {
	var b strings.Builder
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + v.Int64()))
	}
	return b.String(), nil
}

// digitSegments maps each digit to its lit segments (a..g, see renderPNG).
var digitSegments = map[byte][7]bool{
	'0': {true, true, true, true, true, true, false},
	'1': {false, true, true, false, false, false, false},
	'2': {true, true, false, true, true, false, true},
	'3': {true, true, true, true, false, false, true},
	'4': {false, true, true, false, false, true, true},
	'5': {true, false, true, true, false, true, true},
	'6': {true, false, true, true, true, true, true},
	'7': {true, true, true, false, false, false, false},
	'8': {true, true, true, true, true, true, true},
	'9': {true, true, true, true, false, true, true},
}

const (
	imgW = 150
	imgH = 52
)

// renderPNG draws the answer as four seven-segment digits over a noisy
// background. Standard library only, on purpose: no font or image deps.
func renderPNG(answer string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{245, 245, 240, 255}}, image.Point{}, draw.Src)

	r := mrand.New(mrand.NewSource(time.Now().UnixNano()))

	// noise speckles
	for i := 0; i < 120; i++ {
		x, y := r.Intn(imgW), r.Intn(imgH)
		g := uint8(200 + r.Intn(40))
		img.Set(x, y, color.RGBA{g, g, g, 255})
	}

	// faint random lines
	for i := 0; i < 3; i++ {
		drawLine(img, r.Intn(imgW), r.Intn(imgH), r.Intn(imgW), r.Intn(imgH), color.RGBA{180, 185, 190, 255})
	}

	const (
		digW = 24
		digH = 40
		segT = 5
		gap  = 10
	)
	x0 := (imgW - (len(answer)*digW + (len(answer)-1)*gap)) / 2
	y0 := (imgH - digH) / 2

	for i := 0; i < len(answer); i++ {
		segments := digitSegments[answer[i]]
		c := color.RGBA{
			R: uint8(30 + r.Intn(60)),
			G: uint8(50 + r.Intn(60)),
			B: uint8(100 + r.Intn(80)),
			A: 255,
		}
		bx := x0 + i*(digW+gap)
		drawDigit(img, bx, y0, segments, c)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func drawDigit(img *image.RGBA, bx, by int, seg [7]bool, c color.Color) {
	const (
		w = 24
		h = 40
		t = 5
	)
	half := by + h/2
	rect := func(x0, y0, x1, y1 int) {
		draw.Draw(img, image.Rect(x0, y0, x1, y1), &image.Uniform{c}, image.Point{}, draw.Src)
	}
	if seg[0] { // a: top
		rect(bx+t, by, bx+w-t, by+t)
	}
	if seg[1] { // b: upper right
		rect(bx+w-t, by+t, bx+w, half)
	}
	if seg[2] { // c: lower right
		rect(bx+w-t, half, bx+w, by+h-t)
	}
	if seg[3] { // d: bottom
		rect(bx+t, by+h-t, bx+w-t, by+h)
	}
	if seg[4] { // e: lower left
		rect(bx, half, bx+t, by+h-t)
	}
	if seg[5] { // f: upper left
		rect(bx, by+t, bx+t, half)
	}
	if seg[6] { // g: middle
		rect(bx+t, half-t/2, bx+w-t, half+t-t/2)
	}
}

// drawLine sets pixels along a Bresenham line (1px wide, faint).
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx := 1
	if x0 >= x1 {
		sx = -1
	}
	sy := 1
	if y0 >= y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.Set(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
