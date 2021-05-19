package cupboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/anthhub/taskgroup"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type Result struct {
	Host        string // The host IP; eg: 127.0.0.1
	BindingPort string // The The port of the host binding the container
	URI         string // The URI to connect the container; eg: 127.0.0.1:2017
}

type Option struct {
	Override    bool     // Override container when the name of the container is duplicated
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

type payload struct {
	Ret    *Result
	Cancel func()
	Index  int
}

// It is to handle multiple containers.
func WithContainers(ctx context.Context, options []*Option) (rets []*Result, cancel func(), err error) {
	length := len(options)
	rets = make([]*Result, length)
	canceles := make([]func(), length)

	g, c := taskgroup.WithContext(ctx, &taskgroup.Option{MaxErrorCount: 1})
	for i, o := range options {
		i := i
		o := o
		g.Go(func() (interface{}, error) {
			ret, cancel, err := WithContainer(c, o)
			if err != nil {
				return nil, err
			}
			return &payload{ret, cancel, i}, nil
		})
	}

	for p := range g.Fed().Result() {
		err = p.Err
		if err != nil {
			return
		}
		data, ok := (p.Data).(*payload)
		if !ok {
			panic(err)
		}

		rets[data.Index] = data.Ret
		canceles[data.Index] = data.Cancel
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

	err = checkImage(ctx, c, option)
	if err != nil {
		return
	}

	err = checkContainer(ctx, c, option)
	if err != nil {
		return
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
		err := forceRemoveContainer(ctx, c, containerID)
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

func forceRemoveContainer(ctx context.Context, c *client.Client, containerID string) error {
	err := c.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return err
	}
	return nil
}

func checkImage(ctx context.Context, c *client.Client, option *Option) error {

	images, err := c.ImageList(ctx, types.ImageListOptions{})
	if err != nil {
		return err
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
			return err
		}
		io.Copy(os.Stdout, reader)
	}

	return nil
}

func checkContainer(ctx context.Context, c *client.Client, option *Option) error {

	if !option.Override {
		return nil
	}

	containers, err := c.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return err
	}
	for _, container := range containers {
		for _, name := range container.Names {
			name = strings.Replace(name, "/", "", 1)
			cname := strings.Replace(option.Name, "/", "", 1)

			if name == cname {
				err := forceRemoveContainer(ctx, c, container.ID)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}
