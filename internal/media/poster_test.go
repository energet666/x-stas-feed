package media

import "testing"

func TestPosterFrameUsableRejectsBlackFrame(t *testing.T) {
	frame := solidRGBFrame(64, 64, 0, 0, 0)

	if posterFrameUsable(frame) {
		t.Fatal("expected black frame to be rejected")
	}
}

func TestPosterFrameUsableAcceptsBrightFrame(t *testing.T) {
	frame := solidRGBFrame(64, 64, 80, 120, 160)

	if !posterFrameUsable(frame) {
		t.Fatal("expected visible frame to be accepted")
	}
}

func TestPosterFrameUsableRejectsDimFlatFrame(t *testing.T) {
	frame := solidRGBFrame(64, 64, 45, 45, 45)

	if posterFrameUsable(frame) {
		t.Fatal("expected dim flat frame to be rejected")
	}
}

func TestPosterFrameUsableAcceptsDarkFrameWithBrightSubject(t *testing.T) {
	frame := solidRGBFrame(64, 64, 20, 20, 20)
	for i := 0; i < len(frame)/5; i += 3 {
		frame[i] = 180
		frame[i+1] = 180
		frame[i+2] = 180
	}

	if !posterFrameUsable(frame) {
		t.Fatal("expected dark frame with bright subject to be accepted")
	}
}

func TestPosterFrameUsableRejectsMostlyBlackFrame(t *testing.T) {
	frame := solidRGBFrame(64, 64, 0, 0, 0)
	for i := 0; i < len(frame)/10; i += 3 {
		frame[i] = 240
		frame[i+1] = 240
		frame[i+2] = 240
	}

	if posterFrameUsable(frame) {
		t.Fatal("expected mostly black frame to be rejected")
	}
}

func TestPosterFrameFallbackScoreAcceptsDarkDetailedFrame(t *testing.T) {
	frame := solidRGBFrame(64, 64, 4, 4, 4)
	for y := 8; y < 56; y += 4 {
		for x := 8; x < 56; x++ {
			setRGBPixel(frame, 64, x, y, 180, 180, 180)
		}
	}

	if posterFrameUsable(frame) {
		t.Fatal("expected dark detailed frame not to pass primary filter")
	}
	if posterFrameFallbackScore(frame) <= 0 {
		t.Fatal("expected dark detailed frame to be a fallback candidate")
	}
}

func TestPosterFrameFallbackScoreRejectsDimBlurredFrame(t *testing.T) {
	frame := solidRGBFrame(64, 64, 20, 20, 20)

	if posterFrameFallbackScore(frame) > 0 {
		t.Fatal("expected dim blurred frame not to be a fallback candidate")
	}
}

func solidRGBFrame(width, height int, r, g, b byte) []byte {
	frame := make([]byte, width*height*3)
	for i := 0; i+2 < len(frame); i += 3 {
		frame[i] = r
		frame[i+1] = g
		frame[i+2] = b
	}
	return frame
}

func setRGBPixel(frame []byte, width, x, y int, r, g, b byte) {
	i := (y*width + x) * 3
	frame[i] = r
	frame[i+1] = g
	frame[i+2] = b
}
