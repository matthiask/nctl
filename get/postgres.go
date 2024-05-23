package get

import (
	"context"
	"fmt"
	"io"
	"text/tabwriter"

	storage "github.com/ninech/apis/storage/v1alpha1"
	"github.com/ninech/nctl/api"
	"github.com/ninech/nctl/internal/format"
)

type postgresCmd struct {
	Name          string `arg:"" help:"Name of the PostgreSQL instance to get. If omitted all in the project will be listed." default:""`
	PrintPassword bool   `help:"Print the password of the PostgreSQL User. Requires name to be set." xor:"print"`
	PrintUser     bool   `help:"Print the name of the PostgreSQL User. Requires name to be set." xor:"print"`

	out io.Writer
}

func (cmd *postgresCmd) Run(ctx context.Context, client *api.Client, get *Cmd) error {
	cmd.out = defaultOut(cmd.out)

	if cmd.Name != "" && cmd.PrintUser {
		fmt.Fprintln(cmd.out, storage.PostgresUser)
		return nil
	}

	postgresList := &storage.PostgresList{}

	if err := get.list(ctx, client, postgresList, matchName(cmd.Name)); err != nil {
		return err
	}

	if len(postgresList.Items) == 0 {
		printEmptyMessage(cmd.out, storage.PostgresKind, client.Project)
		return nil
	}

	if cmd.Name != "" && cmd.PrintPassword {
		return cmd.printPassword(ctx, client, &postgresList.Items[0])
	}

	switch get.Output {
	case full:
		return cmd.printPostgresInstances(postgresList.Items, get, true)
	case noHeader:
		return cmd.printPostgresInstances(postgresList.Items, get, false)
	case yamlOut:
		return format.PrettyPrintObjects(postgresList.GetItems(), format.PrintOpts{})
	}

	return nil
}

func (cmd *postgresCmd) printPostgresInstances(list []storage.Postgres, get *Cmd, header bool) error {
	w := tabwriter.NewWriter(cmd.out, 0, 0, 4, ' ', 0)

	if header {
		get.writeHeader(w, "NAME", "FQDN", "LOCATION", "MACHINE TYPE")
	}

	for _, postgres := range list {
		get.writeTabRow(w, postgres.Namespace, postgres.Name, postgres.Status.AtProvider.FQDN, string(postgres.Spec.ForProvider.Location), string(postgres.Spec.ForProvider.MachineType))
	}

	return w.Flush()
}

func (cmd *postgresCmd) printPassword(ctx context.Context, client *api.Client, postgres *storage.Postgres) error {
	pw, err := getConnectionSecret(ctx, client, storage.PostgresUser, postgres)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, pw)
	return nil
}
