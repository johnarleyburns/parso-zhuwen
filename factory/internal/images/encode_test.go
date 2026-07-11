package images

import (
	"bytes"
	"testing"

	"github.com/parso/zhuwen-factory/internal/pack"
)

func TestEncodeStubDeterministic(t *testing.T) {
	cfg := DefaultEncodeConfig()
	a, _, _, err := EncodeImage("img1", "File:Test.jpg", "https://commons.wikimedia.org/wiki/File:Test.jpg", cfg)
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, err := EncodeImage("img1", "File:Test.jpg", "https://commons.wikimedia.org/wiki/File:Test.jpg", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("stub image bytes not deterministic for same image ID")
	}
}

func TestEncodeStubDistinctPerImage(t *testing.T) {
	cfg := DefaultEncodeConfig()
	a, _, _, _ := EncodeImage("img1", "", "", cfg)
	b, _, _, _ := EncodeImage("img2", "", "", cfg)
	if bytes.Equal(a, b) {
		t.Fatal("distinct image IDs must yield distinct stub bytes")
	}
}

func TestEncodeStubCompact(t *testing.T) {
	cfg := DefaultEncodeConfig()
	data, _, _, err := EncodeImage("img1", "", "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) >= RepresentativeHeicByteLen {
		t.Fatalf("default stub (%d B) should be smaller than representative (%d B)", len(data), RepresentativeHeicByteLen)
	}
	// Must be a valid PNG (starts with PNG signature).
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(data, pngSig) {
		t.Fatal("stub must be valid PNG data")
	}
}

func TestEncodeStubRepresentative(t *testing.T) {
	cfg := DefaultEncodeConfig()
	cfg.RepresentativeStub = true
	data, _, _, err := EncodeImage("img1", "", "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != RepresentativeHeicByteLen {
		t.Fatalf("representative stub = %d B, want %d B", len(data), RepresentativeHeicByteLen)
	}
	// Must still be a valid PNG.
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	if !bytes.HasPrefix(data, pngSig) {
		t.Fatal("representative stub must be valid PNG data")
	}
}

func TestEncodeRealModeRequiresConfig(t *testing.T) {
	cfg := DefaultEncodeConfig()
	cfg.Mode = EncodeModeReal
	_, _, _, err := EncodeImage("img1", "", "", cfg)
	if err == nil {
		t.Fatal("EncodeModeReal without PythonBin/Script must error")
	}
}

func TestEncodePackImagesPopulatesData(t *testing.T) {
	imgs := []pack.Image{
		{ID: "img1", File: "images/img1@480.heic"},
		{ID: "img2", File: "images/img2@480.heic"},
	}
	encoded, err := EncodePackImages(imgs, DefaultEncodeConfig())
	if err != nil {
		t.Fatal(err)
	}
	for i, im := range encoded {
		if im.Data == nil {
			t.Fatalf("image %d (%s): Data is nil after encode", i, im.ID)
		}
		if len(im.Data) == 0 {
			t.Fatalf("image %d (%s): Data is empty after encode", i, im.ID)
		}
	}
}

func TestEncodePackImagesSkipsPopulated(t *testing.T) {
	preload := []byte("real-heic-bytes")
	imgs := []pack.Image{
		{ID: "img1", File: "images/img1@480.heic", Data: preload},
		{ID: "img2", File: "images/img2@480.heic"},
	}
	encoded, err := EncodePackImages(imgs, DefaultEncodeConfig())
	if err != nil {
		t.Fatal(err)
	}
	// First image should retain its pre-populated Data.
	if !bytes.Equal(encoded[0].Data, preload) {
		t.Fatal("image with pre-populated Data was overwritten")
	}
	// Second image should get stub Data.
	if encoded[1].Data == nil {
		t.Fatal("image without Data was not populated")
	}
}

func TestEncodeStubDimensions(t *testing.T) {
	cfg := DefaultEncodeConfig()
	cfg.TargetPX = 480
	_, w, h, err := EncodeImage("img1", "", "", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if w != 480 || h != 480 {
		t.Fatalf("stub dimensions = %d×%d, want 480×480", w, h)
	}
}
