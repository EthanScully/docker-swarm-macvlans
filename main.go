package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

func main() {
	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		panic(fmt.Errorf("error connecting to docker socket: %v", err))
	}
	defer cli.Close()
	repeat := 15
	if len(os.Args) > 1 {
		repeat, err = strconv.Atoi(os.Args[1])
		if err != nil {
			repeat = 15
			fmt.Println("invalid repeat value, using default")
		}
	}
	go func() {
		for {
			err := changeIPs(cli, ctx)
			if err != nil {
				fmt.Println(err)
			}
			time.Sleep(time.Duration(repeat) * time.Second)
		}
	}()
	inter := make(chan os.Signal, 1)
	signal.Notify(inter, os.Interrupt, syscall.SIGTERM)
	<-inter
}
func changeIPs(cli *client.Client, ctx context.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%v", r)
		}
	}()
	// Parse config.json
	configJson, err := os.ReadFile("config.json")
	if err != nil {
		return fmt.Errorf("error reading config.json: %v", err)
	}
	var config any
	err = json.Unmarshal(configJson, &config)
	if err != nil {
		return fmt.Errorf("error parsing json: %v", err)
	}
	stacks := config.(map[string]any)
	services := make(map[string]string)
	for k, v := range stacks {
		srvs := v.(map[string]any)
		for service, network := range srvs {
			name := fmt.Sprintf("%s_%s", k, service)
			services[name] = network.(string)
		}
	}
	// Link network:IP to real container ID
	containers, err := cli.ContainerList(ctx, container.ListOptions{})
	if err != nil {
		return fmt.Errorf("error getting container list: %v", err)
	}
	for _, container := range containers {
		for _, name := range container.Names {
			for service, network := range services {
				if strings.Contains(name, service) {
					delete(services, service)
					services[container.ID] = network
				}
			}
		}
	}
	for containerID, networkIP := range services {
		// Get Container object
		container, err := cli.ContainerInspect(ctx, containerID)
		if err != nil {
			fmt.Printf("error inspecting container: %v\n", err)
			break
		}
		// split network name and ip from json
		temp := strings.Split(networkIP, ":")
		networkID, IP := temp[0], temp[1]
		// Get network ID
		networks, err := cli.NetworkList(ctx, network.ListOptions{})
		if err != nil {
			fmt.Printf("error getting networks: %v\n", err)
			break
		}
		for _, network := range networks {
			if network.Name == networkID {
				networkID = network.ID
				break
			}
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
				err = cli.NetworkConnect(ctx, networkID, containerID, &network.EndpointSettings{IPAMConfig: &network.EndpointIPAMConfig{IPv4Address: IP}})
				if err != nil {
					fmt.Printf("error connecting to network: %v, using random IP\n", err)
					cli.NetworkConnect(ctx, networkID, containerID, &network.EndpointSettings{})
					break
				}
				fmt.Printf("Changing service: %v to %v\n", container.Name, IP)
			}
			break
		}
		break
	}
	return
}
