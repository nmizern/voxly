package media

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
)

func TestExtractAudio(t *testing.T) {
	if !Available() {
		t.Skip("ffmpeg not installed")
	}

	gen := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-f", "lavfi", "-i", "testsrc=size=64x64:rate=15:duration=1",
		"-shortest", "-c:v", "libvpx", "-c:a", "libopus", "-f", "webm", "pipe:1",
	)
	var input bytes.Buffer
	gen.Stdout = &input
	if err := gen.Run(); err != nil {
		t.Skipf("cannot generate test video: %v", err)
	}

	out, err := ExtractAudio(context.Background(), input.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(out) == 0 {
		t.Fatal("expected audio output")
	}
}
