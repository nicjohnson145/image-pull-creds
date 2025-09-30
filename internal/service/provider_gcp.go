package service

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2/google"
)

type ProviderGCPConfig struct {
	CredsJSON  []byte
	Registries []string
}

func NewProviderGCP(conf ProviderGCPConfig) (*ProviderGCP, error) {
	if len(conf.CredsJSON) == 0 {
		return nil, fmt.Errorf("JSON credentials are required")
	}
	if len(conf.Registries) == 0 {
		return nil, fmt.Errorf("must supply at least one registry")
	}

	return &ProviderGCP{
		credsJSON:  conf.CredsJSON,
		registries: conf.Registries,
	}, nil
}

type ProviderGCP struct {
	credsJSON  []byte
	registries []string

	creds *google.Credentials
}

func (p *ProviderGCP) ensureCreds(ctx context.Context) error {
	if p.creds != nil {
		return nil
	}

	creds, err := google.CredentialsFromJSON(ctx, p.credsJSON)
	if err != nil {
		return fmt.Errorf("error parsing credentials: %w", err)
	}

	p.creds = creds

	return nil
}

func (p *ProviderGCP) getToken(ctx context.Context) (string, error) {
	if err := p.ensureCreds(ctx); err != nil {
		return "", fmt.Errorf("error ensuring credentials: %w", err)
	}

	token, err := p.creds.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("error getting token: %w", err)
	}

	return token.AccessToken, nil
}

func (p *ProviderGCP) CreateDockerCFG(ctx context.Context) ([]byte, error) {
	token, err := p.getToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("error getting token for config: %w", err)
	}

	cfg := map[string]any{}
	for _, registry := range p.registries {
		cfg[registry] = map[string]any{
			"username": "oauth2accesstoken",
			"password": token,
			"email": "none",
		}
	}

	cfgBytes, err := json.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("error marshalling: %w", err)
	}

	return cfgBytes, nil
}
