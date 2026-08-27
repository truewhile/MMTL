package model

import "time"

const (
	ScrapeTaskPending  = "pending"
	ScrapeTaskRunning  = "running"
	ScrapeTaskDone     = "done"
	ScrapeTaskFailed   = "failed"
	ScrapeTaskCanceled = "canceled"
)

// ScrapeTask 表示一条持久化的媒体刮削任务。
type ScrapeTask struct {
	Base
	MediaID        string     `gorm:"index;size:36" json:"media_id"`
	LibraryID      string     `gorm:"index;size:36" json:"library_id"`
	LibraryName    string     `gorm:"size:128" json:"library_name"`
	MediaTitle     string     `gorm:"size:255;not null" json:"media_title"`
	MediaPath      string     `gorm:"size:1024;not null" json:"media_path"`
	MediaType      string     `gorm:"size:16" json:"media_type"`                   // movie / tv / anime / adult
	Provider       string     `gorm:"size:32" json:"provider"`                     // tmdb / douban / bangumi / thetvdb / metatube
	MatchedTitle   string     `gorm:"size:255" json:"matched_title"`
	MatchedYear    int        `json:"matched_year"`
	PosterURL      string     `gorm:"size:1024" json:"poster_url"`
	BackdropURL    string     `gorm:"size:1024" json:"backdrop_url"`
	Status         string     `gorm:"index;size:16;default:pending" json:"status"` // pending / running / done / failed / canceled
	Error          string     `gorm:"type:text" json:"error"`
	RetryCount     int        `gorm:"default:0" json:"retry_count"`
	EpisodeImages  bool       `gorm:"default:true" json:"episode_images"`
	RefreshMatched bool       `gorm:"default:false" json:"refresh_matched"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
}
