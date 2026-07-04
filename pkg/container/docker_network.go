//go:build !(WITHOUT_DOCKER || !(linux || darwin || windows || netbsd))

package container

import (
	"context"
	"strings"

	"github.com/moby/moby/client"
	"github.com/nektos/act/pkg/common"
)

func NewDockerNetworkCreateExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		// Only create the network if it doesn't exist
		networks, err := cli.NetworkList(ctx, client.NetworkListOptions{})
		if err != nil {
			return err
		}
		common.Logger(ctx).Debugf("%v", networks)
		for _, network := range networks.Items {
			if network.Name == name {
				common.Logger(ctx).Debugf("Network %v exists", name)
				return nil
			}
		}

		_, err = cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{
			Driver: "bridge",
		})
		if err != nil {
			// Podman (and some other runtimes) may not recognise the bridge driver by that
			// name, or may require no driver to be specified to use their default. Retry
			// without an explicit driver before giving up.
			if strings.Contains(err.Error(), "driver") || strings.Contains(err.Error(), "plugin") {
				common.Logger(ctx).Debugf("Network creation with bridge driver failed (%v), retrying with runtime default", err)
				_, err = cli.NetworkCreate(ctx, name, client.NetworkCreateOptions{})
			}
		}

		return err
	}
}

func NewDockerNetworkRemoveExecutor(name string) common.Executor {
	return func(ctx context.Context) error {
		cli, err := GetDockerClient(ctx)
		if err != nil {
			return err
		}
		defer cli.Close()

		// Make sure that all network of the specified name are removed
		// cli.NetworkRemove refuses to remove a network if there are duplicates
		networks, err := cli.NetworkList(ctx, client.NetworkListOptions{})
		if err != nil {
			return err
		}
		common.Logger(ctx).Debugf("%v", networks)
		for _, net := range networks.Items {
			if net.Name == name {
				result, err := cli.NetworkInspect(ctx, net.ID, client.NetworkInspectOptions{})
				if err != nil {
					return err
				}

				if len(result.Network.Containers) == 0 {
					if _, err = cli.NetworkRemove(ctx, net.ID, client.NetworkRemoveOptions{}); err != nil {
						common.Logger(ctx).Debugf("%v", err)
					}
				} else {
					common.Logger(ctx).Debugf("Refusing to remove network %v because it still has active endpoints", name)
				}
			}
		}

		return err
	}
}
