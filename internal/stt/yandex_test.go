package stt

import "testing"

func TestExtractText(t *testing.T) {
	r := &yandexResult{}
	r.Chunks = append(r.Chunks, struct {
		Alternatives []struct {
			Text string `json:"text"`
		} `json:"alternatives"`
	}{Alternatives: []struct {
		Text string `json:"text"`
	}{{Text: "  привет "}, {Text: "мир"}}})

	if got := extractText(r); got != "привет мир" {
		t.Errorf("extractText = %q, want %q", got, "привет мир")
	}
	if got := extractText(nil); got != "" {
		t.Errorf("extractText(nil) = %q, want empty", got)
	}
}

func TestEncodingForExt(t *testing.T) {
	cases := map[string]string{
		".ogg":   "OGG_OPUS",
		".OGG":   "OGG_OPUS",
		".mp3":   "MP3",
		".wav":   "LINEAR16_PCM",
		".weird": "OGG_OPUS",
	}
	for ext, want := range cases {
		if got := encodingForExt(ext); got != want {
			t.Errorf("encodingForExt(%q) = %q, want %q", ext, got, want)
		}
	}
}
