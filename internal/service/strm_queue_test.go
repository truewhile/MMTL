package service

import (
	"context"
	"errors"
	"testing"

	"github.com/ShukeBta/MMTL/internal/model"
	"github.com/ShukeBta/MMTL/internal/repository"
	"go.uber.org/zap"
)

func TestIs115Blocked(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("115 接口返回 HTTP 405：<!doctypehtml>...访问被阻断"), true},
		{errors.New("115 接口返回 HTTP 405"), true},
		{errors.New("115 接口错误（770004）：访问频率过高"), true},
		{errors.New("115 接口错误（406）：达到访问上限"), true},
		{errors.New("下载失败：http 403"), false},
		{errors.New("解析下载地址失败：115 接口调用失败"), false},
		{errors.New("database is locked (517)"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := is115Blocked(c.err); got != c.want {
			t.Errorf("is115Blocked(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestIsHTTPDownloadFailure(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{errors.New("http 403"), true},
		{errors.New("http 404"), true},
		{errors.New("http 410"), true},
		{errors.New("http 500"), true},
		{errors.New("Get \"https://x\": connection refused"), false},
		{nil, false},
	}
	for _, c := range cases {
		if got := isHTTPDownloadFailure(c.err); got != c.want {
			t.Errorf("isHTTPDownloadFailure(%v) = %v, want %v", c.err, got, c.want)
		}
	}
}

func TestStrmUploadTasksClearAndRetry(t *testing.T) {
	db := newServiceTestDB(t, &model.StrmUploadTask{})
	repos := repository.New(db)
	svc := NewStrmService(nil, zap.NewNop(), repos, nil)
	ctx := context.Background()

	tasks := []*model.StrmUploadTask{
		{Base: model.Base{ID: "task-pending"}, Status: model.StrmTaskPending, FileName: "1.nfo"},
		{Base: model.Base{ID: "task-running"}, Status: model.StrmTaskRunning, FileName: "2.nfo"},
		{Base: model.Base{ID: "task-done"}, Status: model.StrmTaskDone, FileName: "3.nfo"},
		{Base: model.Base{ID: "task-failed"}, Status: model.StrmTaskFailed, FileName: "4.nfo", Error: "some error", RetryCount: 3},
		{Base: model.Base{ID: "task-canceled"}, Status: model.StrmTaskCanceled, FileName: "5.nfo"},
	}
	for _, task := range tasks {
		if err := db.Create(task).Error; err != nil {
			t.Fatalf("failed to insert task: %v", err)
		}
	}

	// 1. RetryAllFailedUploadTasks
	retried, err := svc.RetryAllFailedUploadTasks(ctx)
	if err != nil {
		t.Fatalf("RetryAllFailedUploadTasks failed: %v", err)
	}
	if retried != 1 {
		t.Fatalf("expected 1 retried task, got %d", retried)
	}
	var failedTask model.StrmUploadTask
	if err := db.First(&failedTask, "id = ?", "task-failed").Error; err != nil {
		t.Fatalf("failed to get task-failed: %v", err)
	}
	if failedTask.Status != model.StrmTaskPending || failedTask.Error != "" || failedTask.RetryCount != 0 {
		t.Fatalf("task-failed was not reset properly: %+v", failedTask)
	}

	// 再次改为 failed 以便测试 ClearFinished
	db.Model(&model.StrmUploadTask{}).Where("id = ?", "task-failed").Updates(map[string]any{"status": model.StrmTaskFailed})

	// 2. ClearFinishedUploadTasks 应删除 done, failed, canceled 三条历史记录
	deleted, err := svc.ClearFinishedUploadTasks(ctx)
	if err != nil {
		t.Fatalf("ClearFinishedUploadTasks failed: %v", err)
	}
	if deleted != 3 {
		t.Fatalf("expected 3 deleted tasks (done, failed, canceled), got %d", deleted)
	}

	// 验证剩余的任务只有 pending 和 running
	var count int64
	db.Model(&model.StrmUploadTask{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 remaining tasks, got %d", count)
	}
}
