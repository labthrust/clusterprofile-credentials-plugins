# clusterprofile-credentials-plugins

This repository hosts **Credentials plugins for ClusterProfile**.

Currently it includes:

* **eks-aws-auth-plugin**: An authentication plugin for Amazon EKS (fetches IAM-based tokens)
* **examples/**: Sample usage of the plugin

---

## Go implementation: eks-aws-auth-plugin

### Build

```bash
# Build single binary
GOOS=$(go env GOOS) GOARCH=$(go env GOARCH) go build -o bin/eks-aws-auth-plugin ./cmd/eks-aws-auth-plugin

# Example: Cross compile
GOOS=linux GOARCH=amd64 go build -o bin/eks-aws-auth-plugin-linux-amd64 ./cmd/eks-aws-auth-plugin
```

### Usage

This binary replicates the behavior of `eks-aws-auth-plugin.sh`.

* Requires the AWS CLI (`aws`) configured with credentials and permissions to call EKS APIs
* Reads `KUBERNETES_EXEC_INFO` (kubeconfig `exec` with `provideClusterInfo: true`)
* Infers region from the EKS API server endpoint
* Resolves cluster name from endpoint using a tiny cache at `${XDG_CACHE_HOME:-$HOME/.cache}/eks-exec-credential/endpoint-map-<region>.json`
* Emits ExecCredential JSON to stdout

Example kubeconfig snippet:

```yaml
users:
  - name: eks-aws-auth
    user:
      exec:
        apiVersion: client.authentication.k8s.io/v1beta1
        command: /path/to/eks-aws-auth-plugin
        provideClusterInfo: true
```

### Notes

* The binary depends on `aws` CLI being available in `PATH`.
* No `jq` dependency is required.

---

## Release (GitHub Actions)

Tag a version and push it. The workflow builds for multiple OS/ARCH, archives, generates checksums, and creates a GitHub Release.

```bash
git tag v0.1.0
git push origin v0.1.0
```

Artifacts:

* linux/amd64, linux/arm64: `tar.gz`
* darwin/amd64, darwin/arm64: `tar.gz`
* windows/amd64, windows/arm64: `zip`
* `checksums.txt` with SHA256 hashes
