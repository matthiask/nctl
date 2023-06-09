package test

import (
	"os"

	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/auth"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func SetupClient(initObjs ...client.Object) (*api.Client, error) {
	scheme, err := api.NewScheme()
	if err != nil {
		return nil, err
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(initObjs...).Build()

	return &api.Client{
		WithWatch: client, Namespace: "default",
	}, nil
}

// CreateTestKubeconfig creates a test kubeconfig which contains a nctl
// extension config with the given organization
func CreateTestKubeconfig(client *api.Client, organization string) (string, error) {
	contextName := "test"
	cfg := auth.NewConfig(organization)
	cfgObject, err := cfg.ToObject()
	if err != nil {
		return "", err
	}
	// create and open a temporary file
	f, err := os.CreateTemp("", "kubeconfig-")
	if err != nil {
		return "", err
	}
	defer f.Close()

	kubeconfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			contextName: {
				Server: "not.so.important",
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			contextName: {
				Token: "not-valid",
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			contextName: {
				Cluster:   contextName,
				AuthInfo:  contextName,
				Namespace: "default",
				Extensions: map[string]runtime.Object{
					auth.NctlExtensionName: cfgObject,
				},
			},
		},
		CurrentContext: contextName,
	}

	content, err := clientcmd.Write(kubeconfig)
	if err != nil {
		return "", err
	}
	if _, err = f.Write(content); err != nil {
		return "", err
	}
	client.KubeconfigContext = contextName
	client.KubeconfigPath = f.Name()

	return f.Name(), nil
}
