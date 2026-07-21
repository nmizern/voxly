package stt

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAICompatible_Transcribe(t *testing.T) {
	var gotAuth, gotModel, gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/audio/transcriptions") {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = r.ParseMultipartForm(1 << 20)
		gotModel = r.FormValue("model")
		gotLang = r.FormValue("language")
		w.Write([]byte(`{"text":"привет мир"}`))
	}))
	defer srv.Close()

	c := NewOpenAICompatible("openai", srv.URL, "sk-test", "gpt-4o-transcribe", "ru-RU")
	res, err := c.Transcribe(context.Background(), Audio{
		Data:     strings.NewReader("audiobytes"),
		Filename: "voice.ogg",
		MIME:     "audio/ogg",
	})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	if res.Text != "привет мир" {
		t.Errorf("text = %q", res.Text)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotModel != "gpt-4o-transcribe" {
		t.Errorf("model = %q", gotModel)
	}
	if gotLang != "ru" { // normalized from ru-RU
		t.Errorf("language = %q, want ru", gotLang)
	}
}

func TestOpenAICompatible_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		io.WriteString(w, `{"error":"bad key"}`)
	}))
	defer srv.Close()

	c := NewOpenAICompatible("groq", srv.URL, "x", "whisper-large-v3-turbo", "")
	_, err := c.Transcribe(context.Background(), Audio{Data: strings.NewReader("x"), Filename: "a.ogg"})
	if err == nil || !strings.Contains(err.Error(), "status=401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}
