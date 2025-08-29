package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"sigs.k8s.io/aws-iam-authenticator/pkg/token"
)

// Utilities
func errPrintf(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "[eks-exec-credential] "+format+"\n", a...)
}

// (no external binary dependencies)

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

// ExecCredential input structures (partial)
type execInfo struct {
	APIVersion string `json:"apiVersion"`
	Spec       struct {
		Cluster struct {
			Server string `json:"server"`
		} `json:"cluster"`
	} `json:"spec"`
}

func readExecInfo() (*execInfo, error) {
	val := os.Getenv("KUBERNETES_EXEC_INFO")
	if strings.TrimSpace(val) == "" {
		return nil, errors.New("KUBERNETES_EXEC_INFO is empty. set provideClusterInfo: true")
	}
	var info execInfo
	if err := json.Unmarshal([]byte(val), &info); err != nil {
		return nil, fmt.Errorf("failed to parse KUBERNETES_EXEC_INFO: %w", err)
	}
	if strings.TrimSpace(info.Spec.Cluster.Server) == "" || info.Spec.Cluster.Server == "null" {
		return nil, errors.New("spec.cluster.server is missing in KUBERNETES_EXEC_INFO")
	}
	return &info, nil
}

func inferRegionFromServer(server string) (string, error) {
	norm, err := normalizeHost(server)
	if err != nil {
		return "", err
	}
	host := norm
	if idx := strings.Index(norm, "/"); idx >= 0 {
		host = norm[:idx]
	}
	re := regexp.MustCompile(`.*\.([a-z0-9-]+)\.eks(-fips)?\.amazonaws\.com(\.cn)?$`)
	m := re.FindStringSubmatch(host)
	if len(m) == 0 {
		return "", fmt.Errorf("failed to parse region from server hostname: %s", server)
	}
	return m[1], nil
}

// Simple JSON map cache endpoint->cluster name
func cacheDir() (string, error) {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(h, ".cache")
	}
	dir := filepath.Join(base, "eks-exec-credential")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func mapCachePath(region string) (string, error) {
	dir, err := cacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fmt.Sprintf("endpoint-map-%s.json", region)), nil
}

func readCache(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func writeCache(path string, m map[string]string) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func normalizeFromServer(server string) (string, error) {
	return normalizeHost(server)
}

func matchEndpoint(region, clusterName, normServer string) (bool, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return false, nil
	}
	client := eks.NewFromConfig(cfg)
	out, err := client.DescribeCluster(ctx, &eks.DescribeClusterInput{Name: aws.String(clusterName)})
	if err != nil {
		return false, nil // tolerate errors to allow enumeration path
	}
	if out.Cluster == nil || out.Cluster.Endpoint == nil {
		return false, nil
	}
	norm, err := normalizeHost(aws.ToString(out.Cluster.Endpoint))
	if err != nil {
		return false, nil
	}
	return norm == normServer, nil
}

func listClusters(region string) ([]string, error) {
	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, err
	}
	client := eks.NewFromConfig(cfg)
	var names []string
	p := eks.NewListClustersPaginator(client, &eks.ListClustersInput{})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		names = append(names, page.Clusters...)
	}
	return names, nil
}

func getTokenJSON(region, clusterName string) ([]byte, error) {
	ctx := context.Background()
	// token generator uses SDK default credentials chain and region
	gen, err := token.NewGenerator(true, false)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize token generator: %w", err)
	}
	opts := &token.GetTokenOptions{
		ClusterID: clusterName,
		Region:    region,
	}
	t, err := gen.GetWithOptions(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get EKS token: %w", err)
	}
	// Build ExecCredential-style JSON similar to aws CLI output
	var out struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
		Spec       struct{} `json:"spec"`
		Status     struct {
			ExpirationTimestamp string `json:"expirationTimestamp"`
			Token               string `json:"token"`
		} `json:"status"`
	}
	out.Kind = "ExecCredential"
	// Use v1 to be compatible with modern kubectl; many clients also accept v1beta1
	out.APIVersion = "client.authentication.k8s.io/v1"
	out.Status.Token = t.Token
	if !t.Expiration.IsZero() {
		out.Status.ExpirationTimestamp = t.Expiration.UTC().Format(time.RFC3339Nano)
	}
	return json.Marshal(out)
}

func main() {
	// Strict-ish: exit non-zero on error

	info, err := readExecInfo()
	if err != nil {
		errPrintf("%v", err)
		os.Exit(1)
	}

	normServer, err := normalizeFromServer(info.Spec.Cluster.Server)
	if err != nil {
		errPrintf("%v", err)
		os.Exit(1)
	}

	region, err := inferRegionFromServer(info.Spec.Cluster.Server)
	if err != nil {
		errPrintf("%v", err)
		errPrintf("expected something like ...<random>.<suffix>.<region>.eks.amazonaws.com")
		os.Exit(1)
	}

	cachePath, err := mapCachePath(region)
	if err != nil {
		errPrintf("%v", err)
		os.Exit(1)
	}

	cacheMap, _ := readCache(cachePath)
	clusterName := ""
	if cached, ok := cacheMap[normServer]; ok {
		okMatch, _ := matchEndpoint(region, cached, normServer)
		if okMatch {
			clusterName = cached
		}
	}

	if clusterName == "" {
		errPrintf("resolving cluster in %s for %s", region, normServer)
		names, err := listClusters(region)
		if err != nil {
			errPrintf("%v", err)
			os.Exit(1)
		}
		for _, name := range names {
			if strings.TrimSpace(name) == "" {
				continue
			}
			okMatch, _ := matchEndpoint(region, name, normServer)
			if okMatch {
				clusterName = name
				break
			}
		}
		if clusterName == "" {
			errPrintf("no matching EKS cluster for endpoint: %s (region=%s)", info.Spec.Cluster.Server, region)
			os.Exit(1)
		}
		cacheMap[normServer] = clusterName
		_ = writeCache(cachePath, cacheMap)
	}

	b, err := getTokenJSON(region, clusterName)
	if err != nil {
		errPrintf("%v", err)
		os.Exit(1)
	}

	// Output exactly as the aws CLI returns (JSON)
	w := bufio.NewWriter(os.Stdout)
	_, _ = w.Write(b)
	_ = w.WriteByte('\n')
	_ = w.Flush()
}
