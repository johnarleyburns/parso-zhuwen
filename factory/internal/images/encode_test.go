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
		t.Fatal("stub HEIC bytes not deterministic for same image ID")
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
	// The stub marker prefix must be present.
	if !bytes.HasPrefix(data, []byte("HeicZhuwenStub\x00")) {
		t.Fatal("stub must contain the marker prefix")
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
