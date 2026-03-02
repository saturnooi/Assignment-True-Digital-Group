package domain

import "time"

type User struct {
	ID           int64
	Age          int
	Country      string
	Subscription string
}

type Content struct {
	ID              int64
	Title           string
	Genre           string
	PopularityScore float64
	CreatedAt       time.Time
}

type WatchRecord struct {
	ContentID int64
	Genre     string
	WatchedAt time.Time
}
