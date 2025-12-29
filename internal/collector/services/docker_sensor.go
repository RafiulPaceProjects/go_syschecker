package services

import (
	"context"
	"encoding/json"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/shirou/gopsutil/v4/docker"
)

type DockerContainerStat struct {
	ID         string
	Name       string
	Image      string
	Status     string
	Running    bool
	CPUUsage   float64 // Total CPU usage in seconds (or ticks, depending on OS)
	MemUsage   uint64
	MemLimit   uint64
	MemPercent float64
}

type DockerResult struct {
	Available  bool
	Containers []DockerContainerStat
}

type DockerSensor struct{}

func NewDockerSensor() *DockerSensor {
	return &DockerSensor{}
}

func (s *DockerSensor) Name() string {
	return "Docker"
}

func (s *DockerSensor) Connect(ctx context.Context) error {
	return nil
}

func (s *DockerSensor) Disconnect(ctx context.Context) error {
	return nil
}

func (s *DockerSensor) Collect(ctx context.Context) (any, error) {
	// On macOS, gopsutil's Docker module doesn't work well with Docker Desktop
	// Use Docker CLI as the primary method on macOS
	if runtime.GOOS == "darwin" {
		return s.collectViaCLI(ctx)
	}

	// On Linux, try gopsutil first (uses cgroups, more efficient)
	containers, err := docker.GetDockerStatWithContext(ctx)
	if err != nil {
		// Fallback to CLI method
		return s.collectViaCLI(ctx)
	}

	var results []DockerContainerStat

	for _, c := range containers {
		stat := DockerContainerStat{
			ID:      c.ContainerID,
			Name:    c.Name,
			Image:   c.Image,
			Status:  c.Status,
			Running: c.Running,
		}

		if c.Running {
			// Attempt to get Memory stats
			if mem, err := docker.CgroupMemDockerWithContext(ctx, c.ContainerID); err == nil {
				stat.MemUsage = mem.MemUsageInBytes
				stat.MemLimit = mem.MemLimitInBytes
				if stat.MemLimit > 0 {
					stat.MemPercent = float64(stat.MemUsage) / float64(stat.MemLimit) * 100.0
				}
			}

			// Attempt to get CPU stats
			if cpu, err := docker.CgroupCPUDockerWithContext(ctx, c.ContainerID); err == nil {
				stat.CPUUsage = cpu.Usage
			}
		}

		results = append(results, stat)
	}

	return DockerResult{
		Available:  true,
		Containers: results,
	}, nil
}

// collectViaCLI uses Docker CLI to get container info (works on macOS)
func (s *DockerSensor) collectViaCLI(ctx context.Context) (DockerResult, error) {
	// Check if Docker is available by running `docker info`
	checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	checkCmd := exec.CommandContext(checkCtx, "docker", "info", "--format", "{{.ServerVersion}}")
	if err := checkCmd.Run(); err != nil {
		return DockerResult{Available: false}, nil
	}

	// Docker is available, get container list
	listCtx, listCancel := context.WithTimeout(ctx, 10*time.Second)
	defer listCancel()

	// Use docker ps with JSON format for parsing
	listCmd := exec.CommandContext(listCtx, "docker", "ps", "-a", "--format", "{{json .}}")
	output, err := listCmd.Output()
	if err != nil {
		// Docker is available but couldn't list containers (permissions?)
		return DockerResult{Available: true, Containers: nil}, nil
	}

	var containers []DockerContainerStat
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		var cInfo struct {
			ID     string `json:"ID"`
			Names  string `json:"Names"`
			Image  string `json:"Image"`
			Status string `json:"Status"`
			State  string `json:"State"`
		}

		if err := json.Unmarshal([]byte(line), &cInfo); err != nil {
			continue
		}

		containers = append(containers, DockerContainerStat{
			ID:      cInfo.ID,
			Name:    cInfo.Names,
			Image:   cInfo.Image,
			Status:  cInfo.Status,
			Running: cInfo.State == "running",
		})
	}

	return DockerResult{
		Available:  true,
		Containers: containers,
	}, nil
}
