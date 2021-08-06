package cupboard

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/anthhub/taskgroup"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// It is to handle multiple containers.
func WithContainers(ctx context.Context, options []*Option) (*Result, error) {
	length := len(options)
	infos := make([]*Info, length)
	canceles := make([]func(), length)

	g, c := taskgroup.WithContext(ctx, &taskgroup.Option{MaxErrorCount: 1})
	for i, o := range options {
		i := i
		o := o
		g.Go(func() (interface{}, error) {
			info, cancel, err := upContainer(c, o)
			if err != nil {
				return nil, err
			}
			return &payload{info, cancel, i}, nil
		})
	}

	for p := range g.Fed().Result() {
		err := p.Err
		if err != nil {
			return nil, err
		}
		data, ok := (p.Data).(*payload)
		if !ok {
			panic(err)
		}

		infos[data.Index] = data.Info
		canceles[data.Index] = data.Cancel
	}

	cancel := func() {
		for _, c := range canceles {
			if c != nil {
				c()
			}
		}
	}

	ret := &Result{
		Infos: infos,
		Close: cancel,
	}

	ret.Wait = func() {
		fmt.Println("Wait...")
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		<-sigs
		fmt.Println("Bye...")
		ret.Close()
	}

	go func() {
		<-ctx.Done()
		ret.Close()
	}()

	return ret, nil
}

// It is to handle one container.
//
// It will create a container from an image provided; it will pull the image if the image is unavailable in local.
//
// If you want to delete the container, you can call the cancel function.
func upContainer(ctx context.Context, option *Option) (ret *Info, cancel func(), err error) {
	defer func() {
		if err != nil && cancel != nil {
			cancel()
		}
	}()

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

	var once sync.Once
	cancel = func() {
		once.Do(func() {
			err := forceRemoveContainer(ctx, c, containerID)
			if err != nil {
				panic(err)
			}
		})
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
	ret = &Info{URI: URI, Host: hostPort.HostIP, BindingPort: hostPort.HostPort}
	return
}

// It is to force remove the container.
func forceRemoveContainer(ctx context.Context, c *client.Client, containerID string) error {
	err := c.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
		Force: true,
	})
	if err != nil {
		return err
	}
	return nil
}

// It is to find the image from local.
//
// If the image is unavailable, cupboard will pull it.
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

// It is to check the container by name.
//
// If the name of container is exist and option.Override is true, cupboard will remove the container and create a new one.
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

// it to check the options.
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
