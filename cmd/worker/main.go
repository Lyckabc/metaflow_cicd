package main

import (
	"log"
	"os"
	"strings"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	hostPort := os.Getenv("TEMPORAL_ADDRESS")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}
	if strings.HasPrefix(hostPort, "localhost:") {
		hostPort = "127.0.0.1" + hostPort[len("localhost"):]
	}
	// 1. Temporal 서버 연결
	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalln("Unable to create client", err)
	}
	defer c.Close()

	// 2. Worker 설정 (동시 실행 제한은 WorkerTuner로 설정 가능)
	w := worker.New(c, "ci-task-queue", worker.Options{})

	// 3. Workflow 및 Activity 등록
	registerWorkflowsAndActivities(w)

	// 4. 실행
	err = w.Run(worker.InterruptCh())
	if err != nil {
		log.Fatalln("Unable to start worker", err)
	}
}