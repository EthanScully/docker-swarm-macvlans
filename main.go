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
	go func() {
		for {
			update(services)
			time.Sleep(time.Second * 15)
		}
	}()
	inter := make(chan os.Signal, 1)
	signal.Notify(inter, os.Interrupt, syscall.SIGTERM)
	<-inter
}
func update(services []any) {
	go func() {
		r := recover()
		if r != nil {
			fmt.Printf("panic: %v\n", r)
		}
	}()
	for _, service := range services {
		network := service.(map[string]any)["net"].(string)
		name := service.(map[string]any)["name"].(string)
		ip := service.(map[string]any)["ip"].(string)
		cmd := exec.Command("docker", "container", "ls", "-q", "-f", fmt.Sprintf("name=%s", name))
		cmdPrint(cmd.Args)
		output, err := cmd.CombinedOutput()
		fmt.Println(string(output))
		if err != nil {
			panic(err)
		}
		id := string(output)
		l := 12
		if len(id) < l {
			continue
		}
		id = id[:l]
		cmd = exec.Command("docker", "network", "inspect", network)
		cmdPrint(cmd.Args)
		output, err = cmd.CombinedOutput()
		if err != nil {
			panic(err)
		}
		var data any
		err = json.Unmarshal(output, &data)
		if err != nil {
			panic(err)
		}
		containers := data.([]any)[0].(map[string]any)["Containers"].(map[string]any)
		var found bool
		for k, v := range containers {
			if strings.Contains(k, id) {
				found = true
				currentIP := v.(map[string]any)["IPv4Address"].(string)
				if !strings.Contains(currentIP, ip) {
					cmd = exec.Command("docker", "network", "disconnect", network, id)
					cmdPrint(cmd.Args)
					output, err = cmd.CombinedOutput()
					fmt.Println(string(output))
					if err != nil {
						panic(err)
					}
					cmd = exec.Command("docker", "network", "connect", "--ip", ip, network, id)
					cmdPrint(cmd.Args)
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
			cmdPrint(cmd.Args)
			output, err = cmd.CombinedOutput()
			fmt.Println(string(output))
			if err != nil {
				panic(err)
			}
		}
	}
}
func cmdPrint(cmd []string) {
	list := make([]any, len(cmd))
	for i, c := range cmd {
		list[i] = c
	}
	fmt.Println(list...)
}
