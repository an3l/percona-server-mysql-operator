package mysqlsh

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	k8sexec "k8s.io/utils/exec"
)

type mysqlsh struct {
	uri  string
	exec k8sexec.Interface
}

func New(e k8sexec.Interface, uri string) *mysqlsh {
	return &mysqlsh{exec: e, uri: uri}
}

func (m *mysqlsh) run(ctx context.Context, cmd string) error {
	args := []string{"--uri", m.uri, "-e", cmd}

	c := m.exec.CommandContext(ctx, "mysqlsh", args...)

	if err := c.Run(); err != nil {
		return errors.Wrapf(err, "run %s", cmd)
	}

	return nil
}

func (m *mysqlsh) ConfigureInstance(ctx context.Context) error {
	cmd := fmt.Sprintf("dba.configureInstance('%s', {'interactive': false})", m.uri)

	if err := m.run(ctx, cmd); err != nil {
		return errors.Wrap(err, "configure instance")
	}

	return nil
}

func (m *mysqlsh) CreateCluster(ctx context.Context, clusterName string) error {
	cmd := fmt.Sprintf("dba.createCluster('%s', {'adoptFromGR': true})", clusterName)

	if err := m.run(ctx, cmd); err != nil {
		return errors.Wrap(err, "create cluster")
	}

	return nil
}

func (m *mysqlsh) DoesClusterExists(ctx context.Context, clusterName string) bool {
	cmd := fmt.Sprintf("dba.getCluster('%s').status()", clusterName)
	err := m.run(ctx, cmd)
	return err == nil
}

func (m *mysqlsh) ClusterRescan(ctx context.Context, clusterName string) error {
	cmd := fmt.Sprintf("dba.getCluster('%s').rescan()", clusterName)

	if err := m.run(ctx, cmd); err != nil {
		return errors.Wrap(err, "rescan cluster")
	}

	return nil
}
