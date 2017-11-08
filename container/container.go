package container

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/docker/docker/client"
)

func Profile(c *client.Client, containerID, filename string) {
	var measurements []ContainerStats
	for {
		stats, err := c.ContainerStats(context.Background(), containerID,
			false)
		if err != nil {
			fmt.Println(err)
			return
		}
		b, err := ioutil.ReadAll(stats.Body)
		if err != nil {
			fmt.Println(err)
			return
		}
		stats.Body.Close()
		containerStats := ContainerStats{}

		err = json.Unmarshal(b, &containerStats)
		if err != nil {
			fmt.Println(err)
			return
		}
		if containerStats.CPUStats.CPUUsage.TotalUsage != 0 {
			measurements = append(measurements, containerStats)
			b, err = json.Marshal(measurements)
			if err != nil {
				fmt.Println("Could not marshal json", err)
				return
			}
			err = ioutil.WriteFile(filename, b, 0666)
			if err != nil {
				fmt.Println("Could not write profile file", err)
				return
			}
		}

	}
}
