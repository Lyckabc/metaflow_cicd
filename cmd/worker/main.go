package main

import (
	"log"
	"os"
	"strings"
	"sync"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const (
	taskQueue     = "ci-task-queue"
	namespaceCI   = "metaflow-ci"
	namespaceCD   = "metaflow-cd"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}
	if strings.HasPrefix(hostPort, "localhost:") {
		hostPort = "127.0.0.1" + hostPort[len("localhost"):]
	}

	// 1. Temporal 클라이언트 생성 (metaflow-ci, metaflow-cd 네임스페이스)
	cCI, err := client.Dial(client.Options{HostPort: hostPort, Namespace: namespaceCI})
	if err != nil {
		log.Fatalln("Unable to create client (metaflow-ci):", err)
	}
	defer cCI.Close()

	cCD, err := client.Dial(client.Options{HostPort: hostPort, Namespace: namespaceCD})
	if err != nil {
		log.Fatalln("Unable to create client (metaflow-cd):", err)
	}
	defer cCD.Close()

	// 2. Worker 생성 (CI/CD 각각)
	wCI := worker.New(cCI, taskQueue, worker.Options{})
	wCD := worker.New(cCD, taskQueue, worker.Options{})

	// 3. Workflow 및 Activity 등록
	registerWorkflowsAndActivities(wCI)
	registerWorkflowsAndActivities(wCD)

	// 4. 두 Worker 동시 실행
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := wCI.Run(worker.InterruptCh()); err != nil {
			log.Printf("Worker (metaflow-ci) stopped: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := wCD.Run(worker.InterruptCh()); err != nil {
			log.Printf("Worker (metaflow-cd) stopped: %v", err)
		}
	}()
	wg.Wait()
}