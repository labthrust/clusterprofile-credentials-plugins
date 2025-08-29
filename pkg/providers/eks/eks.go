package eks

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/labthrust/clusterprofile-credentials-plugins/pkg/core"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

type Provider struct{}

func (Provider) Name() string { return "eks" }

// normalizeHost converts a URL like https://example.com:443/ to example.com
func normalizeHost(raw string) (string, error) {
	if raw == "" {
		return "", errors.New("empty host")
	}
	// Strip scheme if present
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		u, err := url.Parse(raw)
		if err == nil {
			raw = u.Host + u.Path
		}
	}
	// Drop trailing slash
	raw = strings.TrimSuffix(raw, "/")
	// Drop :443 suffix if present
	raw = strings.TrimSuffix(raw, ":443")
	return raw, nil
}

func (Provider) GetTokenJSON(ctx context.Context, info *clientauthv1beta1.ExecCredential) ([]byte, error) {
	// infer region from server
	norm, err := normalizeHost(info.Spec.Cluster.Server)
	if err != nil {
		return nil, err
	}
	host := norm
	if idx := strings.Index(norm, "/"); idx >= 0 {
		host = norm[:idx]
	}
	re := regexp.MustCompile(`.*\.([a-z0-9-]+)\.eks(-fips)?\.amazonaws\.com(\.cn)?$`)
	m := re.FindStringSubmatch(host)
	if len(m) == 0 {
		return nil, fmt.Errorf("failed to parse region from server hostname: %s", info.Spec.Cluster.Server)
	}
	region := m[1]

	// list and match
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := eks.NewFromConfig(cfg)

	var target string
	p := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, name := range page.Clusters {
			out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(name)})
			if err != nil || out.Cluster == nil || out.Cluster.Endpoint == nil {
				continue
			}
			n, nErr := normalizeHost(aws.ToString(out.Cluster.Endpoint))
			if nErr != nil || n != norm {
				continue
			}
			if len(info.Spec.Cluster.CertificateAuthorityData) > 0 {
				if out.Cluster.CertificateAuthority == nil || out.Cluster.CertificateAuthority.Data == nil {
					continue
				}
				decoded, decErr := base64.StdEncoding.DecodeString(aws.ToString(out.Cluster.CertificateAuthority.Data))
				if decErr != nil || !bytes.Equal(decoded, info.Spec.Cluster.CertificateAuthorityData) {
					continue
				}
			}
			target = name
			break
		}
		if target != "" {
			break
		}
	}
	if target == "" {
		return nil, fmt.Errorf("no matching EKS cluster for endpoint: %s (region=%s)", info.Spec.Cluster.Server, region)
	}

	// mint token
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token generator: %w", err)
	}
	opts := &token.GetTokenOptions{ClusterID: target, Region: region}
	t, err := gen.GetWithOptions(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get EKS token: %w", err)
	}
	return core.BuildExecCredentialJSON(t.Token, t.Expiration)
}
