package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/truewhile/MeBox/internal/model"
	"github.com/truewhile/MeBox/internal/repository"
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

func TestDownloadSemSeparateByProvider(t *testing.T) {
	svc := NewStrmService(nil, zap.NewNop(), nil, nil)
	svc.initDownloadSems(6)

	if cap(svc.downloadSem115) != strm115DownloadSemCap {
		t.Fatalf("115 sem cap = %d, want %d", cap(svc.downloadSem115), strm115DownloadSemCap)
	}
	if cap(svc.downloadSemDAV) != 6 {
		t.Fatalf("dav sem cap = %d, want 6", cap(svc.downloadSemDAV))
	}

	ctx := context.Background()
	for i := 0; i < strm115DownloadSemCap; i++ {
		if !svc.acquireDownloadSlot(ctx, model.StrmProvider115) {
			t.Fatalf("failed to acquire 115 slot %d", i)
		}
	}
	// 115 槽位占满后，OpenList 仍应能独立获取槽位。
	if !svc.acquireDownloadSlot(ctx, model.StrmProviderOpenList) {
		t.Fatal("openlist slot should remain available while 115 slots are full")
	}
	for i := 0; i < 5; i++ {
		if !svc.acquireDownloadSlot(ctx, model.StrmProviderOpenList) {
			t.Fatalf("failed to acquire extra openlist slot %d", i)
		}
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if svc.acquireDownloadSlot(timeoutCtx, model.StrmProviderOpenList) {
		t.Fatal("expected openlist slots to be exhausted after 6 acquisitions")
	}
}

func TestRequeueDownloadTask(t *testing.T) {
	db := newServiceTestDB(t, &model.StrmDownloadTask{})
	repos := repository.New(db)
	svc := NewStrmService(nil, zap.NewNop(), repos, nil)
	now := time.Now()
	task := &model.StrmDownloadTask{
		Base:      model.Base{ID: "dl-running"},
		Status:    model.StrmTaskRunning,
		Provider:  model.StrmProviderOpenList,
		StartedAt: &now,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatalf("insert task: %v", err)
	}

	svc.requeueDownloadTask(task)

	var got model.StrmDownloadTask
	if err := db.First(&got, "id = ?", task.ID).Error; err != nil {
		t.Fatalf("load task: %v", err)
	}
	if got.Status != model.StrmTaskPending || got.StartedAt != nil {
		t.Fatalf("task not requeued: %+v", got)
	}
}
