package bookmark

import "time"

type Bookmark struct {
	Title     string
	Text      string
	URL       string
	DateAdded time.Time
}
