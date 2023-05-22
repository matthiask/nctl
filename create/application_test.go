package create

import (
	"context"
	"testing"
	"time"

	runtimev1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	apps "github.com/ninech/apis/apps/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestApplication(t *testing.T) {
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	apiClient := &api.Client{WithWatch: client, Namespace: "default"}
	ctx := context.Background()

	cases := map[string]struct {
		cmd      applicationCmd
		checkApp func(t *testing.T, cmd applicationCmd, app *apps.Application)
	}{
		"without git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					SubPath:  "/my/app",
					Revision: "superbug",
				},
				Wait:     false,
				Name:     "custom-name",
				Size:     "mini",
				Hosts:    []string{"custom.example.org", "custom2.example.org"},
				Port:     1337,
				Replicas: 42,
				Env:      map[string]string{"hello": "world"},
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				assert.Equal(t, cmd.Name, app.Name)
				assert.Equal(t, cmd.Git.URL, app.Spec.ForProvider.Git.URL)
				assert.Equal(t, cmd.Git.SubPath, app.Spec.ForProvider.Git.SubPath)
				assert.Equal(t, cmd.Git.Revision, app.Spec.ForProvider.Git.Revision)
				assert.Equal(t, cmd.Hosts, app.Spec.ForProvider.Hosts)
				assert.Equal(t, apps.ApplicationSize(cmd.Size), *app.Spec.ForProvider.Config.Size)
				assert.Equal(t, int32(cmd.Port), *app.Spec.ForProvider.Config.Port)
				assert.Equal(t, int32(cmd.Replicas), *app.Spec.ForProvider.Config.Replicas)
				assert.Equal(t, toEnvVars(cmd.Env), app.Spec.ForProvider.Config.Env)
				assert.Nil(t, app.Spec.ForProvider.Git.Auth)
			},
		},
		"with user/pass git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:      "https://github.com/ninech/doesnotexist.git",
					Username: "deploy",
					Password: "hunter2",
				},
				Wait: false,
				Name: "user-pass-auth",
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				authSecret := gitAuthSecret(cmd.Git, app.Name, app.Namespace)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, cmd.Git.Username, string(authSecret.Data["username"]))
				assert.Equal(t, cmd.Git.Password, string(authSecret.Data["password"]))
			},
		},
		"with ssh key git auth": {
			cmd: applicationCmd{
				Git: gitConfig{
					URL:           "https://github.com/ninech/doesnotexist.git",
					SSHPrivateKey: "fakekey",
				},
				Wait: false,
				Name: "ssh-key-auth",
				Size: "mini",
			},
			checkApp: func(t *testing.T, cmd applicationCmd, app *apps.Application) {
				authSecret := gitAuthSecret(cmd.Git, app.Name, app.Namespace)
				if err := apiClient.Get(ctx, api.ObjectName(authSecret), authSecret); err != nil {
					t.Fatal(err)
				}

				assert.Equal(t, cmd.Git.SSHPrivateKey, string(authSecret.Data["privatekey"]))
			},
		},
	}

	for name, tc := range cases {
		tc := tc
		t.Run(name, func(t *testing.T) {
			app := tc.cmd.newApplication("default")

			if err := tc.cmd.Run(ctx, apiClient); err != nil {
				t.Fatal(err)
			}

			if err := apiClient.Get(ctx, api.ObjectName(app), app); err != nil {
				t.Fatal(err)
			}

			tc.checkApp(t, tc.cmd, app)
		})
	}
}

func TestApplicationWait(t *testing.T) {
	scheme, err := api.NewScheme()
	if err != nil {
		t.Fatal(err)
	}

	cmd := applicationCmd{
		Wait:        true,
		WaitTimeout: time.Second * 5,
		Name:        "some-name",
	}
	namespace := "default"

	build := &apps.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "any-name",
			Namespace: namespace,
			Labels: map[string]string{
				applicationNameLabel: cmd.Name,
			},
		},
	}

	release := &apps.Release{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "another-name",
			Namespace: namespace,
			Labels: map[string]string{
				applicationNameLabel: cmd.Name,
			},
		},
	}

	// throw in a second build/release to ensure it can handle it
	build2 := *build
	build2.Name = build2.Name + "-1"
	release2 := *release
	release2.Name = release2.Name + "-1"

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(build, &build2, release, &release2).Build()
	apiClient := &api.Client{WithWatch: client, Namespace: namespace}

	ctx := context.Background()

	// to test the wait we create a ticker that continously updates our
	// resources in a goroutine to simulate a controller doing the same
	ticker := time.NewTicker(100 * time.Millisecond)
	done := make(chan bool)
	errors := make(chan error, 1)

	go func() {
		for {
			select {
			case <-done:
				close(errors)
				return
			case <-ticker.C:
				app := &apps.Application{}
				if err := apiClient.Get(ctx, types.NamespacedName{Name: cmd.Name, Namespace: namespace}, app); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, app, runtimev1.ReconcileSuccess()); err != nil {
					errors <- err
				}

				app.Status.AtProvider.Hosts = []apps.VerificationStatus{{Name: "host.example.org"}}
				app.Status.AtProvider.CNAMETarget = "some.target.example.org"
				if err := apiClient.Status().Update(ctx, app); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, build, runtimev1.Available()); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &build2, runtimev1.Available()); err != nil {
					errors <- err
				}

				if err := apiClient.Get(ctx, api.ObjectName(build), build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusRunning
				if err := apiClient.Status().Update(ctx, build); err != nil {
					errors <- err
				}

				build.Status.AtProvider.BuildStatus = buildStatusSuccess
				if err := apiClient.Status().Update(ctx, build); err != nil {
					errors <- err
				}

				if err := setResourceCondition(ctx, apiClient, &release2, runtimev1.Available()); err != nil {
					errors <- err
				}

				release.Status.AtProvider.ReleaseStatus = releaseStatusAvailable
				if err := apiClient.Status().Update(ctx, release); err != nil {
					errors <- err
				}
			}
		}
	}()

	if err := cmd.Run(ctx, apiClient); err != nil {
		t.Fatal(err)
	}

	ticker.Stop()
	done <- true

	for err := range errors {
		t.Fatal(err)
	}
}

func setResourceCondition(ctx context.Context, apiClient *api.Client, mg resource.Managed, condition runtimev1.Condition) error {
	if err := apiClient.Get(ctx, api.ObjectName(mg), mg); err != nil {
		return err
	}

	mg.SetConditions(condition)
	return apiClient.Status().Update(ctx, mg)
}