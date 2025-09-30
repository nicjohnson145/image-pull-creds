package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Provider interface {
	CreateDockerCFG(ctx context.Context) ([]byte, error)
}

func NewProviderFromEnv() (Provider, error) {
	kind, err := ParseProviderKind(viper.GetString(ProviderType))
	if err != nil {
		return nil, err
	}

	switch kind {
	case ProviderKindGcp:
		return NewProviderGCP(ProviderGCPConfig{
			CredsJSON:  []byte(viper.GetString(ProviderGCPCredsJSON)),
			Registries: strings.Split(viper.GetString(ProviderGCPRegistries), ","),
		})
	default:
		return nil, fmt.Errorf("unhandled provider kind of %v", kind)
	}
}
