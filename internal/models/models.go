package models

import (
	"time"
)

type FileCollection struct {
	ID          uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	NodeName    string    `gorm:"type:varchar(255);not null;index:idx_node_name" json:"node_name"`
	MountPath   string    `gorm:"type:varchar(500);not null" json:"mount_path"`
	CollectedAt time.Time `gorm:"type:datetime(3);not null;index:idx_collected_at" json:"collected_at"`
	FileCount   int       `gorm:"not null" json:"file_count"`

	Files []CollectedFile `gorm:"foreignKey:CollectionID;constraint:OnDelete:CASCADE" json:"files,omitempty"`
}

func (FileCollection) TableName() string {
	return "file_collections"
}

type CollectedFile struct {
	ID           uint64 `gorm:"primaryKey;autoIncrement" json:"id"`
	CollectionID uint64 `gorm:"not null;index:idx_collection_id" json:"collection_id"`
	FileName     string `gorm:"type:varchar(1000);not null" json:"file_name"`
}

func (CollectedFile) TableName() string {
	return "collected_files"
}

