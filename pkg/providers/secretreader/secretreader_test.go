package secretreader

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	v1alpha1 "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
)

func Test_pickClusterProfileName_ServerAndCA_Match(t *testing.T) {
	g := gomega.NewWithT(t)
	list := &v1alpha1.ClusterProfileList{}
	list.Items = append(list.Items, v1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "cp-1"},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: ProviderName,
					Cluster: clientcmdv1.Cluster{
						Server:                   "https://example.com:443/",
						CertificateAuthorityData: []byte("CA1"),
					},
				},
			},
		},
	})

	// Non-matching item 2
	list.Items = append(list.Items, v1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "cp-2"},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: ProviderName,
					Cluster: clientcmdv1.Cluster{
						Server:                   "https://other.com",
						CertificateAuthorityData: []byte("CA2"),
					},
				},
			},
		},
	})

	name := pickClusterProfileName(list, ProviderName, "example.com", []byte("CA1"))
	g.Expect(name).To(gomega.Equal("cp-1"))

	// CA mismatch should not match
	name = pickClusterProfileName(list, ProviderName, "example.com", []byte("CAx"))
	g.Expect(name).To(gomega.BeEmpty())

	// Empty CA input means server-only check
	name = pickClusterProfileName(list, ProviderName, "example.com", nil)
	g.Expect(name).To(gomega.Equal("cp-1"))
}

func Test_normalizeHost(t *testing.T) {
	g := gomega.NewWithT(t)
	cases := []struct {
		in  string
		out string
	}{{"https://example.com:443/", "example.com"}, {"http://h:443", "h"}, {"h:443/", "h"}, {"h/", "h"}}
	for _, c := range cases {
		got, err := normalizeHost(c.in)
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(got).To(gomega.Equal(c.out))
	}
}

func Test_pickClusterProfileName_NonDefaultPort(t *testing.T) {
	g := gomega.NewWithT(t)
	list := &v1alpha1.ClusterProfileList{}

	// cp with 8443
	list.Items = append(list.Items, v1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "cp-8443"},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: ProviderName,
					Cluster: clientcmdv1.Cluster{
						Server:                   "https://example.com:8443/",
						CertificateAuthorityData: []byte("CAx"),
					},
				},
			},
		},
	})

	// cp with default 443
	list.Items = append(list.Items, v1alpha1.ClusterProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "cp-443"},
		Status: v1alpha1.ClusterProfileStatus{
			CredentialProviders: []v1alpha1.CredentialProvider{
				{
					Name: ProviderName,
					Cluster: clientcmdv1.Cluster{
						Server:                   "https://example.com:443/",
						CertificateAuthorityData: []byte("CAx"),
					},
				},
			},
		},
	})

	// Exact match when specifying non-default port
	name := pickClusterProfileName(list, ProviderName, "example.com:8443", []byte("CAx"))
	g.Expect(name).To(gomega.Equal("cp-8443"))

	// Server-only check still differentiates ports
	name = pickClusterProfileName(list, ProviderName, "example.com:8443", nil)
	g.Expect(name).To(gomega.Equal("cp-8443"))

	// Without port, should resolve to the 443 entry
	name = pickClusterProfileName(list, ProviderName, "example.com", nil)
	g.Expect(name).To(gomega.Equal("cp-443"))
}

// --- e2e-like tests with only dynamic fake client ---

func newExecCredential(server string, ca []byte) *clientauthv1beta1.ExecCredential {
	return &clientauthv1beta1.ExecCredential{
		Spec: clientauthv1beta1.ExecCredentialSpec{
			Cluster: &clientauthv1beta1.Cluster{
				Server:                   server,
				CertificateAuthorityData: ca,
			},
		},
	}
}

func newDynamicWithListKinds(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	gvr := schema.GroupVersionResource{Group: "multicluster.x-k8s.io", Version: "v1alpha1", Resource: "clusterprofiles"}
	listKinds := map[schema.GroupVersionResource]string{
		gvr: "ClusterProfileList",
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objs...)
}

func makeClusterProfileUnstructured(name, ns, server string) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "multicluster.x-k8s.io/v1alpha1",
		"kind":       "ClusterProfile",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"status": map[string]interface{}{
			"credentialProviders": []interface{}{
				map[string]interface{}{
					"name": "secretreader",
					"cluster": map[string]interface{}{
						"server": server,
					},
				},
			},
		},
	}
	return &unstructured.Unstructured{Object: obj}
}

func makeSecretUnstructured(ns, name, token string) *unstructured.Unstructured {
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"type": "Opaque",
	}
	if token != "" {
		obj["data"] = map[string]interface{}{
			"token": base64.StdEncoding.EncodeToString([]byte(token)),
		}
	}
	return &unstructured.Unstructured{Object: obj}
}

func Test_GetTokenJSON_DynamicFake_Success(t *testing.T) {
	g := gomega.NewWithT(t)

	cp := makeClusterProfileUnstructured("cp-1", "ns1", "https://example.com:443/")
	sec := makeSecretUnstructured("ns1", "cp-1", "mytoken")
	dyn := newDynamicWithListKinds(cp, sec)

	p := Provider{Client: dyn, Namespace: "ns1"}
	in := newExecCredential("https://example.com:443/", nil)

	b, err := p.GetTokenJSON(context.Background(), in)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var out clientauthv1beta1.ExecCredential
	g.Expect(json.Unmarshal(b, &out)).To(gomega.Succeed())
	g.Expect(out.Status).NotTo(gomega.BeNil())
	g.Expect(out.Status.Token).To(gomega.Equal("mytoken"))
}

func Test_GetTokenJSON_DynamicFake_NoMatch(t *testing.T) {
	g := gomega.NewWithT(t)

	cp := makeClusterProfileUnstructured("cp-1", "ns1", "https://other.com")
	dyn := newDynamicWithListKinds(cp)

	p := Provider{Client: dyn, Namespace: "ns1"}
	in := newExecCredential("https://example.com:443/", nil)

	_, err := p.GetTokenJSON(context.Background(), in)
	g.Expect(err).To(gomega.HaveOccurred())
}

func Test_GetTokenJSON_DynamicFake_MissingToken(t *testing.T) {
	g := gomega.NewWithT(t)

	cp := makeClusterProfileUnstructured("cp-1", "ns1", "https://example.com")
	sec := makeSecretUnstructured("ns1", "cp-1", "")
	dyn := newDynamicWithListKinds(cp, sec)

	p := Provider{Client: dyn, Namespace: "ns1"}
	in := newExecCredential("https://example.com", nil)

	_, err := p.GetTokenJSON(context.Background(), in)
	g.Expect(err).To(gomega.HaveOccurred())
}
