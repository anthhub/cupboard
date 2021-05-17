package cupboard

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"golang.org/x/sync/errgroup"
)

type Result struct {
	Host        string // The host IP; eg: 127.0.0.1
	BindingPort string // The The port of the host binding the container
	URI         string // The URI to connect the container; eg: 127.0.0.1:2017
}

type Option struct {
	Name        string   // The container name
	Image       string   // The container image and tag; eg: redis:latest
	ExposedPort string   // The exposed port of the container
	BindingPort string   // The port of the host binding the container; if not provide, the cupboard will generate a port randomly
	Protocol    string   // The protocol of connection; default is tcp
	Env         []string // List of environment variable to set in the container
	HostIP      string   // Host IP; default is 127.0.0.1
}

var hostIP = "127.0.0.1"

func checkOption(option *Option) (*Option, error) {

	if option.Image == "" {
		return nil, fmt.Errorf("image is unavailable")
	}

	if option.Protocol == "" {
		option.Protocol = "tcp"
	}

	if option.HostIP == "" {
		option.HostIP = hostIP
	}

	return option, nil
}

// It is to handle multiple containers.
func WithContainers(ctx context.Context, option []*Option) (rets []*Result, cancel func(), err error) {
	var (
		canceles []func()
	)

	var g errgroup.Group
	retCh := make(chan *Result, len(option))
	cancelCh := make(chan func(), len(option))

	for _, o := range option {
		o := o
		g.Go(func() error {
			ret, cancel, err := WithContainer(ctx, o)
			if err != nil {
				return err
			}
			retCh <- ret
			cancelCh <- cancel
			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return
	}

	for range option {
		rets = append(rets, <-retCh)
		canceles = append(canceles, <-cancelCh)
	}

	cancel = func() {
		for _, c := range canceles {
			if c != nil {
				c()
			}
		}
	}

	return
}

// It is to handle one container.
//
// It will create a container from an image provided; it will pull the image if the image is unavailable in local.
//
// If you want to delete the container, you can call the cancel function.
func WithContainer(ctx context.Context, option *Option) (ret *Result, cancel func(), err error) {

	option, err = checkOption(option)
	if err != nil {
		return
	}

	portAndProtocol := option.ExposedPort + "/" + option.Protocol

	c, err := client.NewEnvClient()
	if err != nil {
		return
	}

	images, err := c.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return
	}

	finded := false
	for _, image := range images {
		if finded {
			break
		}
		for _, repoTags := range image.RepoTags {
			if repoTags == option.Image {
				finded = true
				break
			}
		}
	}

	if !finded {
		var reader io.ReadCloser
		reader, err = c.ImagePull(ctx, option.Image, types.ImagePullOptions{})
		if err != nil {
			return
		}
		io.Copy(os.Stdout, reader)
	}

	resp, err := c.ContainerCreate(ctx, &container.Config{
		Image: option.Image,
		ExposedPorts: nat.PortSet{
			nat.Port(portAndProtocol): struct{}{},
		},
		Env: option.Env,
	}, &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(portAndProtocol): []nat.PortBinding{
				{
					HostIP:   option.HostIP,
					HostPort: option.BindingPort,
				},
			},
		},
	}, nil, nil, option.Name)
	if err != nil {
		return
	}
	containerID := resp.ID

	cancel = func() {
		err := c.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
			Force: true,
		})
		if err != nil {
			panic(err)
		}
	}

	err = c.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return
	}

	inspRes, err := c.ContainerInspect(ctx, containerID)
	if err != nil {
		return
	}
	ports := inspRes.NetworkSettings.Ports[nat.Port(portAndProtocol)]

	if len(ports) == 0 {
		err = fmt.Errorf("port is unavailable")
		return
	}

	hostPort := ports[0]
	URI := fmt.Sprintf("%s:%s", hostPort.HostIP, hostPort.HostPort)
	ret = &Result{URI: URI, Host: hostPort.HostIP, BindingPort: hostPort.HostPort}

	return
}
