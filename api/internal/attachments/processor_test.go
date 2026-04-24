package attachments

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func TestBuildPromptCompressesOversizedImageForLLM(t *testing.T) {
	var raw bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 1600, 1600))
	for y := 0; y < 1600; y++ {
		for x := 0; x < 1600; x++ {
			img.Set(x, y, color.RGBA{
				R: uint8((x * y) % 251),
				G: uint8((x + y) % 241),
				B: uint8((x*3 + y*5) % 239),
				A: 255,
			})
		}
	}
	if err := png.Encode(&raw, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	prompt, err := BuildPrompt("Describe this image", []UploadedFile{
		{
			Name:        "test_image.png",
			ContentType: "image/png",
			Data:        raw.Bytes(),
		},
	}, PromptOptions{MaxImageBytes: 200 * 1024})
	if err != nil {
		t.Fatalf("build prompt: %v", err)
	}

	if strings.Contains(prompt, "Ignored oversized images") {
		t.Fatalf("expected image to be compressed, got skipped prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "data:image/jpeg;base64,") {
		t.Fatalf("expected compressed JPEG data URL in prompt")
	}
}
