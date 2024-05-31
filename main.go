package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("error connecting to docker socket: %v", err))
	}
	go func() {
		for {
			err := changeIPs(cli, ctx)
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(15 * time.Second)
		}
	}()
	inter := make(chan os.Signal, 1)
	signal.Notify(inter, os.Interrupt, syscall.SIGTERM)
	<-inter
}
func changeIPs(cli *client.Client, ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = r.(error)
		}
	}()
	services, err := cli.ServiceList(ctx, types.ServiceListOptions{})
	if err != nil {
		return fmt.Errorf("error getting services: %v", err)
	}
	for _, service := range services {
		networkID, IP := containsAlias(service.Spec.TaskTemplate.Networks)
		if IP == "" || networkID == "" {
			continue
		}
		tasks, err := cli.TaskList(ctx, types.TaskListOptions{})
		if err != nil {
			fmt.Printf("error getting tasks: %v\n", err)
			break
		}
		for _, task := range tasks {
			if task.ServiceID != service.ID {
				continue
			}
			if task.Status.State != swarm.TaskStateRunning {
				continue
			}
			_, taskInspect, err := cli.TaskInspectWithRaw(ctx, task.ID)
			if err != nil {
				fmt.Printf("error inspecting task: %v\n", err)
				break
			}
			containerID, err := parseContainerID(taskInspect)
			if err != nil {
				fmt.Printf("error getting containerID: %v\n", err)
				break
			}
			container, err := cli.ContainerInspect(ctx, containerID)
			if err != nil {
				fmt.Printf("error inspecting container: %v\n", err)
			}
			for _, info := range container.NetworkSettings.Networks {
				if info.NetworkID != networkID {
					continue
				}
				if info.IPAddress != IP {
					err := cli.NetworkDisconnect(ctx, networkID, containerID, false)
					if err != nil {
						fmt.Printf("error disconnecting from network: %v\n", err)
						break
					}
					go func(ctx context.Context, networkID, containerID, IP string) {
						var retry int
						for {
							err = cli.NetworkConnect(ctx, networkID, containerID, &network.EndpointSettings{IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: IP}})
							if err == nil  || retry > 60 {
								break
							}
							fmt.Printf("error connecting to network: %v\n", err)
							time.Sleep(time.Second)
							retry++
						}
					}(ctx, networkID, containerID, IP)
					fmt.Printf("Changing service: %v to %v\n", service.Spec.Annotations.Name, IP)
				}
				break
			}
			break
		}
	}
	return
}
func containsAlias(networks []swarm.NetworkAttachmentConfig) (networkID, IP string) {
	for _, network := range networks {
		for _, alias := range network.Aliases {
			if strings.Count(alias, ".") == 3 {
				return network.Target, alias
			}
		}
	}
	return
}
func parseContainerID(taskInspect []byte) (containerID string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	var parsed any
	err = json.Unmarshal(taskInspect, &parsed)
	if err != nil {
		return "", fmt.Errorf("json Unmarshal Error: %v", err)
	}
	containerID = parsed.(map[string]any)["Status"].(map[string]any)["ContainerStatus"].(map[string]any)["ContainerID"].(string)
	return
}
