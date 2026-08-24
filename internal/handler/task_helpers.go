package handler

import "github.com/ShukeBta/MMTL/internal/service"

func finishHTTPTask(task *service.TaskHandle, err error, stage, message string, metrics map[string]int64, details []string) {
	if task == nil {
		return
	}
	task.Finish(err, service.TaskUpdate{Stage: stage, Message: message, Metrics: metrics, Details: details})
}
