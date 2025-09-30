package service

import (
	"strings"

	"github.com/spf13/viper"
)

//go:generate go-enum -f $GOFILE -marshal -names

/*
ENUM(
gcp
)
*/
type ProviderKind string

const (
	LoggingLevel  = "log.level"
	LoggingFormat = "log.format"

	ProviderType          = "provider.type"
	ProviderGCPCredsJSON  = "provider.gcp.creds_json"
	ProviderGCPRegistries = "provider.gcp.registries"
)

var (
	DefaultProviderType          = ProviderKindGcp.String()
	DefaultProviderGCPRegistries = "https://us-docker.pkg.dev"
)

func InitConfig() {
	viper.SetDefault(ProviderType, DefaultProviderType)
	viper.SetDefault(ProviderGCPRegistries, DefaultProviderGCPRegistries)

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
}
