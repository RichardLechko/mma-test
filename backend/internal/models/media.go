package models

type Media struct {
	ID              string `json:"id"`
	FightID         string `json:"fight_id"`
	MediaType       string `json:"media_type"`
	URL             string `json:"url"`
	StorageLocation string `json:"storage_location"`
	ArchiveLocation string `json:"archive_location"`
	IsArchived      bool   `json:"is_archived"`
}