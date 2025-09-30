package service

import (
	"context"
	"fmt"
	"sync"

	"connectrpc.com/connect"
	"github.com/nicjohnson145/hlp/set"
	pbv1 "github.com/nicjohnson145/image-pull-creds/gen/go/image_pull_creds/v1"
	pbv1connect "github.com/nicjohnson145/image-pull-creds/gen/go/image_pull_creds/v1/image_pull_credsv1connect"
	"github.com/nicjohnson145/image-pull-creds/internal/logging"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	imagePullSecretName = "auto-image-pull-creds"
)

type ServiceConfig struct {
	Provider Provider
}

func NewService(conf ServiceConfig) *Service {
	ignoredNamespaces := set.New(
		"kube-system",
		"kube-node-lease",
		"kube-public",
	)
	return &Service{
		provider:          conf.Provider,
		ignoredNamespaces: ignoredNamespaces,
	}
}

type Service struct {
	pbv1connect.UnimplementedImagePullCredsServiceHandler

	mu                sync.Mutex
	provider          Provider
	ignoredNamespaces *set.Set[string]
}

func (s *Service) handleError(ctx context.Context, err error, msg string) error {
	str := "an error occurred"
	if msg != "" {
		str = msg
	}

	log := logging.GetContextLogger(ctx)
	log.Err(err).Msg(str)

	switch true {
	default:
		return err
	}
}

func (s *Service) SetupImagePullCreds(ctx context.Context, req *connect.Request[pbv1.SetupImagePullCredsReqeust]) (*connect.Response[pbv1.SetupImagePUllCredsResponse], error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log := logging.GetContextLogger(ctx)

	log.Debug().Msg("getting k8s client")
	client, err := s.newClient()
	if err != nil {
		return nil, s.handleError(ctx, err, "error getting k8s client")
	}

	log.Debug().Msg("getting docker config from provider")
	cfg, err := s.provider.CreateDockerCFG(ctx)
	if err != nil {
		return nil, s.handleError(ctx, err, "error creating image pull dockerconfig")
	}

	log.Debug().Msg("listing namespaces")
	listNSResp, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, s.handleError(ctx, err, "error listing namespaces")
	}

	var nsFilter func(name string) bool
	if len(req.Msg.Namespaces) > 0 {
		allowList := set.New(req.Msg.Namespaces...)
		nsFilter = func(name string) bool {
			return allowList.Contains(name)
		}
	} else {
		nsFilter = func(name string) bool {
			return !s.ignoredNamespaces.Contains(name)
		}
	}

	for _, ns := range listNSResp.Items {
		if !nsFilter(ns.Name) {
			continue
		}

		log.Debug().Msgf("processing namespace %v", ns.Name)

		log.Debug().Msg("ensuring secret exists and is up to date")
		if err := s.ensureImagePullSecret(ctx, client, ns, cfg); err != nil {
			return nil, s.handleError(ctx, err, "error ensuring image pull secret")
		}

		log.Debug().Msg("listing service accounts")
		saListResp, err := client.CoreV1().ServiceAccounts(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, s.handleError(ctx, err, "error listing service accounts")
		}

		for _, acct := range saListResp.Items {
			log.Debug().Msgf("processing service account %v", acct.Name)

			found := false
			for _, secret := range acct.ImagePullSecrets {
				if secret.Name == imagePullSecretName {
					found = true
					break
				}
			}

			if found {
				log.Debug().Msg("secret already present, continuing")
				continue
			}
			log.Debug().Msg("secret not in image pull secrets, adding")
			acct.ImagePullSecrets = append(acct.ImagePullSecrets, corev1.LocalObjectReference{
				Name: imagePullSecretName,
			})
			_, err := client.CoreV1().ServiceAccounts(ns.Name).Update(ctx, &acct, metav1.UpdateOptions{})
			if err != nil {
				return nil, s.handleError(ctx, err, "error updating service account")
			}
		}
	}

	return connect.NewResponse(&pbv1.SetupImagePUllCredsResponse{}), nil
}

func (s *Service) newClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		c, err := clientcmd.BuildConfigFromFlags("", clientcmd.NewDefaultClientConfigLoadingRules().GetDefaultFilename())
		if err != nil {
			return nil, fmt.Errorf("error reading out-of-cluster config: %w", err)
		}
		cfg = c
	}

	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("error getting clientset: %w", err)
	}

	return clientset, nil
}

func (s *Service) ensureImagePullSecret(ctx context.Context, client kubernetes.Interface, namespace corev1.Namespace, dockercfg []byte) error {
	existing, err := client.CoreV1().Secrets(namespace.Name).Get(ctx, imagePullSecretName, metav1.GetOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("error reading secret: %w", err)
		}
		_, err := client.CoreV1().Secrets(namespace.Name).Create(ctx, &corev1.Secret{
			Type: "kubernetes.io/dockercfg",
			ObjectMeta: metav1.ObjectMeta{
				Name:      imagePullSecretName,
				Namespace: namespace.Name,
			},
			Data: map[string][]byte{
				".dockercfg": dockercfg,
			},
		}, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating secret: %w", err)
		}
		return nil
	}
	existing.Data[".dockercfg"] = dockercfg
	_, err = client.CoreV1().Secrets(namespace.Name).Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("error updating secret: %w", err)
	}

	return nil
}
