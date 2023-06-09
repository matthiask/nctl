package create

import (
	"context"
	"os"
	"testing"
	"time"

	management "github.com/ninech/apis/management/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/test"
	"github.com/stretchr/testify/require"
)

func TestProjects(t *testing.T) {
	ctx := context.Background()
	projectName, organization := "testproject", "evilcorp"
	apiClient, err := test.SetupClient()
	if err != nil {
		t.Fatal(err)
	}
	kubeconfig, err := test.CreateTestKubeconfig(apiClient, organization)
	require.NoError(t, err)
	defer os.Remove(kubeconfig)

	cmd := projectCmd{
		Name:        projectName,
		Wait:        false,
		WaitTimeout: time.Second,
	}

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	if err := apiClient.Get(
		ctx,
		api.NamespacedName(projectName, organization),
		&management.Project{},
	); err != nil {
		t.Fatalf("expected project %q to exist, got: %s", "testproject", err)
	}
}
