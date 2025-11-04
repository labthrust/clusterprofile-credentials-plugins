## ClusterProfile Credentials Plugins

This repository provides ClusterProfile Credentials Provider plugins. Currently includes the following 2 plugins:

- token-secretreader-plugin: Reads `data.token` from Kubernetes Secrets (`<namespace>/<name>`) and returns ExecCredential.
- [`eks-aws-auth-plugin`](cmd/eks-aws-auth-plugin/README.md): Resolves EKS clusters from ClusterProfile's `server/CA`, obtains AWS tokens, and returns ExecCredential.

### Background (Common aspects of KEP and exec flow)

- **KEP-5339: ClusterProfile Plugin for Credentials (SIG-Multicluster)**
  - Design for obtaining authentication information (tokens, etc.) using external plugins in ClusterProfile.
  - Authentication logic is extracted as plugins and executed from controllers.
- **KEP-541: external client-go credential providers (exec)**
  - Each plugin in this repository conforms to the client-go exec plugin input/output specification, receiving `KUBERNETES_EXEC_INFO` as input and returning ExecCredential JSON to standard output.
  - The controller side requires the setting `providers[].execConfig.provideClusterInfo: true` (`KUBERNETES_EXEC_INFO` will be passed).

Reference Documentation (Common)

- KEP-5339 (SIG-Multicluster): `https://github.com/kubernetes/enhancements/tree/master/keps/sig-multicluster/5339-clusterprofile-plugin-credentials`
- ExecCredential input/output specification: `https://kubernetes.io/docs/reference/access-authn-authz/authentication/#input-and-output-formats`

### Terminology

- Consumer: The entity that executes the plugin (e.g., ServiceAccount used by controllers).

### Common Flow (Concept)

1. Controller loads `providers` configuration (e.g., `cp-creds.json`)
2. Controller executes exec plugin (each plugin in this repository)
   - Environment variable: `KUBERNETES_EXEC_INFO` (required, provideClusterInfo: true)
3. Plugin performs processing and returns ExecCredential (JSON) via standard output

### Awesome Plugins (External)

A curated list of useful credential provider plugins that are not included in this repository.

- [traviswt/gke-auth-plugin](https://github.com/traviswt/gke-auth-plugin): Single-binary GKE auth plugin; can be used as an alternative to `gke-gcloud-auth-plugin`.
