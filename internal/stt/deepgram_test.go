package stt

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDeepgram_Transcribe(t *testing.T) {
	var gotAuth, gotModel, gotLang string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotModel = r.URL.Query().Get("model")
		gotLang = r.URL.Query().Get("language")
		w.Write([]byte(`{"results":{"channels":[{"alternatives":[{"transcript":"привет мир"}]}]}}`))
	}))
	defer srv.Close()

	c := NewDeepgram(srv.URL, "dg-key", "nova-2", "ru-RU")
	res, err := c.Transcribe(context.Background(), Audio{Data: strings.NewReader("audio"), MIME: "audio/ogg"})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}

	if res.Text != "привет мир" {
		t.Errorf("text = %q", res.Text)
	}
	if gotAuth != "Token dg-key" {
		t.Errorf("auth = %q", gotAuth)
	}
	if gotModel != "nova-2" {
		t.Errorf("model = %q", gotModel)
	}
	if gotLang != "ru" {
		t.Errorf("language = %q, want ru", gotLang)
	}
}

func TestDeepgram_EmptyResult(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"results":{"channels":[]}}`))
	}))
	defer srv.Close()

	c := NewDeepgram(srv.URL, "x", "nova-2", "")
	res, err := c.Transcribe(context.Background(), Audio{Data: strings.NewReader("x")})
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if res.Text != "" {
		t.Errorf("expected empty text, got %q", res.Text)
	}
}
