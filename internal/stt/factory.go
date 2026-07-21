package stt

import (
	"fmt"
	"voxly/internal/config"
)

// New builds the transcriber selected by cfg. store is only required by
// providers that recognise audio from a URI (Yandex); pass nil otherwise.
func New(cfg *config.Config, store ObjectStore) (Transcriber, error) {
	lang := cfg.STT.Language

	switch cfg.STT.Provider {
	case config.ProviderYandex:
		if store == nil {
			return nil, fmt.Errorf("yandex provider requires object storage")
		}
		y := cfg.STT.Yandex
		return NewYandex(y.APIKey, y.FolderID, y.Model, lang, store), nil
	case config.ProviderOpenAI:
		o := cfg.STT.OpenAI
		return NewOpenAICompatible("openai", o.BaseURL, o.APIKey, o.Model, lang), nil
	case config.ProviderGroq:
		g := cfg.STT.Groq
		return NewOpenAICompatible("groq", g.BaseURL, g.APIKey, g.Model, lang), nil
	case config.ProviderWhisper:
		w := cfg.STT.Whisper
		return NewOpenAICompatible("whisper", w.BaseURL, "", w.Model, lang), nil
	case config.ProviderDeepgram:
		d := cfg.STT.Deepgram
		return NewDeepgram(d.BaseURL, d.APIKey, d.Model, lang), nil
	default:
		return nil, fmt.Errorf("unknown stt provider %q", cfg.STT.Provider)
	}
}
