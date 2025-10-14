package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	clientauthenticationv1 "k8s.io/client-go/pkg/apis/clientauthentication/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	credentialplugin "sigs.k8s.io/cluster-inventory-api/pkg/credentialplugin"
)

func main() {
	credentialplugin.Run(Provider{})
}

type Provider struct {
	KubeClient kubernetes.Interface
}

func (Provider) Name() string { return "kubeconfig-secretreader" }

type execConfig struct {
	SecretName      string `json:"secretName"`
	SecretNamespace string `json:"secretNamespace"`
	Key             string `json:"key"`
}

func buildDefaultClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		kubeconfig := os.Getenv("KUBECONFIG")
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build kube client config: %w", err)
		}
	}
	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}
	return kc, nil
}

func (p Provider) GetToken(ctx context.Context, info clientauthenticationv1.ExecCredential) (clientauthenticationv1.ExecCredentialStatus, error) {
	var err error
	if p.KubeClient == nil {
		p.KubeClient, err = buildDefaultClient()
		if err != nil {
			return clientauthenticationv1.ExecCredentialStatus{}, err
		}
	}

	if info.Spec.Cluster == nil || len(info.Spec.Cluster.Config.Raw) == 0 {
		return clientauthenticationv1.ExecCredentialStatus{}, fmt.Errorf("missing ExecCredential.Spec.Cluster.Config")
	}

	var cfg execConfig
	if err := json.Unmarshal(info.Spec.Cluster.Config.Raw, &cfg); err != nil {
		return clientauthenticationv1.ExecCredentialStatus{}, fmt.Errorf("invalid extensions config: %w", err)
	}
	cfg.SecretName = strings.TrimSpace(cfg.SecretName)
	cfg.SecretNamespace = strings.TrimSpace(cfg.SecretNamespace)
	cfg.Key = strings.TrimSpace(cfg.Key)
	if cfg.SecretName == "" || cfg.SecretNamespace == "" || cfg.Key == "" {
		return clientauthenticationv1.ExecCredentialStatus{}, errors.New("extensions must include secretName, secretNamespace and key")
	}

	sec, err := p.KubeClient.CoreV1().Secrets(cfg.SecretNamespace).Get(ctx, cfg.SecretName, metav1.GetOptions{})
	if err != nil {
		return clientauthenticationv1.ExecCredentialStatus{}, fmt.Errorf("failed to get secret %s/%s: %w", cfg.SecretNamespace, cfg.SecretName, err)
	}
	val, ok := sec.Data[cfg.Key]
	if !ok || len(val) == 0 {
		return clientauthenticationv1.ExecCredentialStatus{}, fmt.Errorf("secret %s/%s missing %q key", cfg.SecretNamespace, cfg.SecretName, cfg.Key)
	}

	return clientauthenticationv1.ExecCredentialStatus{Token: string(val)}, nil
}
