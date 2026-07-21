package stt

import (
	"fmt"
	"voxly/internal/config"
)

// New builds the transcriber selected by cfg. store is only required by
// providers that recognise audio from a URI (Yandex); pass nil otherwise.
func New(cfg *config.Config, store ObjectStore) (Transcriber, error) {
	switch cfg.STT.Provider {
	case config.ProviderYandex:
		if store == nil {
			return nil, fmt.Errorf("yandex provider requires object storage")
		}
		y := cfg.STT.Yandex
		return NewYandex(y.APIKey, y.FolderID, y.Model, cfg.STT.Language, store), nil
	default:
		return nil, fmt.Errorf("stt provider %q is not implemented yet", cfg.STT.Provider)
	}
}
