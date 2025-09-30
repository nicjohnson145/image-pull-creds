package cmd

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"github.com/go-logr/zerologr"
	"github.com/nicjohnson145/connecthelp/codec"
	intercepters "github.com/nicjohnson145/connecthelp/interceptors/server"
	pbv1connect "github.com/nicjohnson145/image-pull-creds/gen/go/image_pull_creds/v1/image_pull_credsv1connect"
	"github.com/nicjohnson145/image-pull-creds/internal/logging"
	"github.com/nicjohnson145/image-pull-creds/internal/service"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/protobuf/encoding/protojson"
)

func Serve() *cobra.Command {
	cobra.OnInitialize(service.InitConfig)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run service",
		PreRun: func(cmd *cobra.Command, args []string) {
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Disable adding v-level to log output, zerolog has levels
			zerologr.VerbosityFieldName = ""

			logger := logging.Init(&logging.LoggingConfig{
				Level:  logging.LogLevel(viper.GetString(service.LoggingLevel)),
				Format: logging.LogFormat(viper.GetString(service.LoggingFormat)),
			})

			reflector := grpcreflect.NewStaticReflector(
				pbv1connect.ImagePullCredsServiceName,
			)

			provider, err := service.NewProviderFromEnv()
			if err != nil {
				logger.Err(err).Msg("error creating provider")
				return err
			}

			srv := service.NewService(service.ServiceConfig{
				Provider: provider,
			})

			mux := http.NewServeMux()

			mux.Handle(pbv1connect.NewImagePullCredsServiceHandler(
				srv,
				connect.WithInterceptors(
					intercepters.NewContextLoggerInterceptor(intercepters.ContextLoggerInterceptorConfig{
						RootLogger:        zerologr.New(&logger),
						NoAttachRequestID: true,
					}),
					intercepters.NewPanicInterceptor(intercepters.PanicInterceptorConfig{}),
					intercepters.NewMethodLoggingInterceptor(intercepters.MethodLoggingInterceptorConfig{}),
				),
				connect.WithCodec(codec.NewProtoJSONCodec(codec.ProtoJSONCodecOpts{
					ProtoJsonOpts: protojson.MarshalOptions{
						UseProtoNames:     true,
						EmitDefaultValues: true,
					},
				})),
			))

			// Reflection routing
			mux.Handle(grpcreflect.NewHandlerV1(reflector))
			mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))

			// TODO: configurable
			port := "8080"
			lis, err := net.Listen("tcp4", ":"+port)
			if err != nil {
				logger.Err(err).Msg("error listening")
				return err
			}

			httpServer := http.Server{
				Addr:              ":" + port,
				Handler:           h2c.NewHandler(mux, &http2.Server{}),
				ReadHeaderTimeout: 3 * time.Second,
			}

			// Setup signal handlers so we can gracefully shutdown
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

			go func() {
				s := <-sigChan
				logger.Info().Msgf("got signal %v, attempting graceful shutdown", s)
				dieCtx, dieCancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer dieCancel()
				_ = httpServer.Shutdown(dieCtx)
			}()

			go func() {
				
			}()

			logger.Info().Msgf("starting server on port %v", port)
			if err := httpServer.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Err(err).Msg("error serving")
				return err
			}

			return nil
		},
	}

	return cmd
}
