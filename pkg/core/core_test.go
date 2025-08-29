package core

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/onsi/gomega"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"
)

func TestBuildExecCredentialJSON_WithExpiration(t *testing.T) {
	g := gomega.NewWithT(t)
	exp := time.Now().Add(10 * time.Minute).UTC()
	b, err := BuildExecCredentialJSON("test-token", exp)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var ec clientauthv1beta1.ExecCredential
	g.Expect(json.Unmarshal(b, &ec)).NotTo(gomega.HaveOccurred())

	g.Expect(ec.TypeMeta.APIVersion).To(gomega.Equal(clientauthv1beta1.SchemeGroupVersion.Identifier()))
	g.Expect(ec.TypeMeta.Kind).To(gomega.Equal("ExecCredential"))
	g.Expect(ec.Status).NotTo(gomega.BeNil())
	g.Expect(ec.Status.Token).To(gomega.Equal("test-token"))
	g.Expect(ec.Status.ExpirationTimestamp).NotTo(gomega.BeNil())
	// Allow small drift due to potential truncation or TZ differences.
	got := ec.Status.ExpirationTimestamp.Time.UTC()
	want := exp.UTC()
	delta := got.Sub(want)
	if delta < 0 {
		delta = -delta
	}
	g.Expect(delta).To(gomega.BeNumerically("<=", time.Second))
}

func TestBuildExecCredentialJSON_WithoutExpiration(t *testing.T) {
	g := gomega.NewWithT(t)
	b, err := BuildExecCredentialJSON("no-exp-token", time.Time{})
	g.Expect(err).NotTo(gomega.HaveOccurred())

	var ec clientauthv1beta1.ExecCredential
	g.Expect(json.Unmarshal(b, &ec)).NotTo(gomega.HaveOccurred())

	g.Expect(ec.Status).NotTo(gomega.BeNil())
	g.Expect(ec.Status.Token).To(gomega.Equal("no-exp-token"))
	g.Expect(ec.Status.ExpirationTimestamp).To(gomega.BeNil())
}

func withEnv(key, value string, fn func()) {
	origVal, had := os.LookupEnv(key)
	os.Setenv(key, value)
	defer func() {
		if had {
			os.Setenv(key, origVal)
		} else {
			os.Unsetenv(key)
		}
	}()
	fn()
}

func TestReadExecInfo_EmptyEnv(t *testing.T) {
	g := gomega.NewWithT(t)
	origVal, had := os.LookupEnv("KUBERNETES_EXEC_INFO")
	if had {
		os.Unsetenv("KUBERNETES_EXEC_INFO")
		defer os.Setenv("KUBERNETES_EXEC_INFO", origVal)
	}

	_, err := readExecInfo()
	g.Expect(err).To(gomega.HaveOccurred())
}

func TestReadExecInfo_InvalidJSON(t *testing.T) {
	g := gomega.NewWithT(t)
	withEnv("KUBERNETES_EXEC_INFO", "not-json", func() {
		_, err := readExecInfo()
		g.Expect(err).To(gomega.HaveOccurred())
	})
}

func TestReadExecInfo_MissingServer(t *testing.T) {
	g := gomega.NewWithT(t)
	// spec.cluster is missing entirely
	payload := `
{
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "kind": "ExecCredential",
  "spec": {}
}`
	withEnv("KUBERNETES_EXEC_INFO", payload, func() {
		_, err := readExecInfo()
		g.Expect(err).To(gomega.HaveOccurred())
	})
}

func TestReadExecInfo_Valid(t *testing.T) {
	g := gomega.NewWithT(t)
	payload := `
{
  "apiVersion": "client.authentication.k8s.io/v1beta1",
  "kind": "ExecCredential",
  "spec": {
    "cluster": {
      "server": "https://example.com"
    }
  }
}`
	withEnv("KUBERNETES_EXEC_INFO", payload, func() {
		info, err := readExecInfo()
		g.Expect(err).NotTo(gomega.HaveOccurred())
		g.Expect(info).NotTo(gomega.BeNil())
		g.Expect(info.Spec.Cluster).NotTo(gomega.BeNil())
		g.Expect(info.Spec.Cluster.Server).To(gomega.Equal("https://example.com"))
	})
}
