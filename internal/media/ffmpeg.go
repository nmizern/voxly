package media

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// ExtractAudio pulls the audio track out of a container such as a video note
// and returns it as ogg/opus, the same format Telegram voice messages use.
func ExtractAudio(ctx context.Context, input []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-i", "pipe:0",
		"-vn",
		"-c:a", "libopus",
		"-ar", "48000",
		"-ac", "1",
		"-f", "ogg",
		"pipe:1",
	)
	cmd.Stdin = bytes.NewReader(input)

	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w: %s", err, stderr.String())
	}
	if out.Len() == 0 {
		return nil, fmt.Errorf("ffmpeg produced no audio")
	}
	return out.Bytes(), nil
}

func Available() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}
