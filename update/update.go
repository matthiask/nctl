package update

import (
	"context"

	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type Cmd struct {
	Application applicationCmd `cmd:"" group:"deplo.io" name:"application" aliases:"app" help:"Update an existing deplo.io Application. (Beta - requires access)"`
}

type updater struct {
	mg         resource.Managed
	client     *api.Client
	kind       string
	updateFunc updateFunc
}

type updateFunc func(current resource.Managed) error

func newUpdater(client *api.Client, mg resource.Managed, kind string, f updateFunc) updater {
	return updater{client: client, mg: mg, kind: kind, updateFunc: f}
}

func (u *updater) Update(ctx context.Context) error {
	if err := u.client.Get(ctx, u.client.Name(u.mg.GetName()), u.mg); err != nil {
		return err
	}

	if err := u.updateFunc(u.mg); err != nil {
		return err
	}

	if err := u.client.Update(ctx, u.mg); err != nil {
		return err
	}

	format.PrintSuccessf("⬆️", "updated %s %q", u.kind, u.mg.GetName())
	return nil
}