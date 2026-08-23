package model

// StorageConfig holds the connection settings for one external storage
// backend (Alist / S3 / WebDAV). Type column makes the row poly-typed
// — Config is a JSON blob whose shape is determined by Type.
//
//	alist  → {server, token}
//	s3     → {endpoint, region, bucket, access_key, secret_key, force_path_style}
//	webdav → {url, username, password}
type StorageConfig struct {
	Base
	Type      string `gorm:"uniqueIndex;size:16;not null" json:"type"`
	Config    string `gorm:"type:text;not null" json:"-"` // ciphertext
	Enabled   bool   `gorm:"default:true" json:"enabled"`
	LastError string `gorm:"size:512" json:"last_error,omitempty"`
}
