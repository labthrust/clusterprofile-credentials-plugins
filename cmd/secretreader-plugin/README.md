# Secret Reader plugin

When executed from a controller, this plugin reads a `token` from a Kubernetes Secret (`<CONSUMER_NAMESPACE>/<CLUSTER_PROFILE_NAME>`) and returns **ExecCredential JSON** to standard output.

This plugin is implemented in compliance with the [Secret Reader plugin](https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/5339-clusterprofile-plugin-credentials#secret-reader-plugin) specification.

For common background and exec input/output specifications, please refer to the `README.md` in the repository root.

## Implementation Status

| Status | Feature |
| :------ | :------ |
| ✅ | Read and return `token` key from Secret |

## Prerequisites

- **Secret placement**: A Secret containing `data.token` (Base64) must exist at `<CONSUMER_NAMESPACE>/<CLUSTER_PROFILE_NAME>`
- **Exec input**: `KUBERNETES_EXEC_INFO` must be provided (`provideClusterInfo: true`). See root `README.md` for details.

## Required RBAC

Grant the following permissions to the Consumer ([term definitions in root README](../../README.md#terminology)).

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: secretreader-clusterprofiles
rules:
- apiGroups: ["multicluster.x-k8s.io"]
  resources: ["clusterprofiles"]
  verbs: ["list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: secretreader-clusterprofiles
subjects:
- kind: ServiceAccount
  name: <CONSUMER_SERVICE_ACCOUNT_NAME>
  namespace: <CONSUMER_NAMESPACE>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: secretreader-clusterprofiles
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: secretreader
  namespace: <CONSUMER_NAMESPACE>
rules:
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: secretreader
  namespace: <CONSUMER_NAMESPACE>
subjects:
- kind: ServiceAccount
  name: <CONSUMER_SERVICE_ACCOUNT_NAME>
  namespace: <CONSUMER_NAMESPACE>
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: secretreader
```

## Build

Execute the following from the repository root (output: `bin/secretreader-plugin`).

```bash
make build
```

## Usage

### 1) Provider Configuration (example: `cp-creds.json`)

```jsonc
{
  "providers": [
    {
      "name": "secretreader",
      "execConfig": {
        "apiVersion": "client.authentication.k8s.io/v1beta1",
        "command": "/path/to/bin/secretreader-plugin",
        "provideClusterInfo": true
      }
    }
  ]
}
```

### 2) Store cluster information in ClusterProfile

Match the `status.credentialProviders.secretreader` in the ClusterProfile with the `name` in the Provider configuration using the string `secretreader`.

```yaml
apiVersion: multicluster.x-k8s.io/v1alpha1
kind: ClusterProfile
metadata:
  name: my-cluster-1
spec:
  displayName: my-cluster-1
  clusterManager:
    name: Any-Cluster-Manager
status:
  credentialProviders:
    secretreader:
      cluster:
        server: https://example.k8s.local
        certificate-authority-data: <BASE64-PEM>
```

### 3) Example of Secret to be read

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-cluster-1
  namespace: <CONSUMER_NAMESPACE>
type: Opaque
data:
  # Base64-encoded token string
  token: bXktYmVhcmVyLXRva2Vu
```

## Troubleshooting

- `failed to get secret <ns>/<name>` → Check Secret name/Namespace/RBAC permissions
- `secret <ns>/<name> missing token key` → Verify that `data.token` exists in the Secret
- `KUBERNETES_EXEC_INFO is empty` → Check if `provideClusterInfo: true` is configured

<!-- Common KEP/exec explanations and links are consolidated in the root README -->
