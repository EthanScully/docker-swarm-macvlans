package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

func main() {
	go loop()
	inter := make(chan os.Signal, 1)
	signal.Notify(inter, os.Interrupt, syscall.SIGTERM)
	<-inter
}
func loop() {
	config := "config.yaml"
	if len(os.Args) > 1 {
		config = os.Args[1]
	}
	configFile, err := os.ReadFile(config)
	if err != nil {
		panic(err)
	}
	var data any
	err = yaml.Unmarshal(configFile, &data)
	if err != nil {
		panic(err)
	}
	services := data.([]any)
	for {
		for _, service := range services {
			network := service.(map[string]any)["net"].(string)
			name := service.(map[string]any)["name"].(string)
			ip := service.(map[string]any)["ip"].(string)
			cmd := exec.Command("docker", "container", "ls", "-q", "-f", fmt.Sprintf("name=%s", name))
			fmt.Println(cmd.Args)
			output, err := cmd.CombinedOutput()
			fmt.Println(string(output))
			if err != nil {
				panic(err)
			}
			id := string(output)
			cmd = exec.Command("docker", "network", "inspect", network)
			fmt.Println(cmd.Args)
			output, err = cmd.CombinedOutput()
			if err != nil {
				panic(err)
			}
			var data any
			err = json.Unmarshal(output, &data)
			if err != nil {
				panic(err)
			}
			l := 12
			id = id[:l]
			containers := data.([]any)[0].(map[string]any)["Containers"].(map[string]any)
			var found bool
			for k, v := range containers {
				if k[:l] == id {
					found = true
					currentIP := v.(map[string]any)["IPv4Address"].(string)
					if !strings.Contains(currentIP, ip) {
						cmd = exec.Command("docker", "network", "disconnect", network, id)
						fmt.Println(cmd.Args)
						output, err = cmd.CombinedOutput()
						fmt.Println(string(output))
						if err != nil {
							panic(err)
						}
						cmd = exec.Command("docker", "network", "connect", "--ip", ip, network, id)
						fmt.Println(cmd.Args)
						output, err = cmd.CombinedOutput()
						fmt.Println(string(output))
						if err != nil {
							panic(err)
						}
					}
				}
			}
			if !found {
				cmd = exec.Command("docker", "network", "connect", "--ip", ip, network, id)
				fmt.Println(cmd.Args)
				output, err = cmd.CombinedOutput()
				fmt.Println(string(output))
				if err != nil {
					panic(err)
				}
			}
		}
		time.Sleep(time.Second * 15)
	}
}
