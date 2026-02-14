package main

import (
	"log"

	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

func main() {
	// 1. Temporal 서버 연결 (기본 localhost:7233)
	c, err := client.Dial(client.Options{})
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