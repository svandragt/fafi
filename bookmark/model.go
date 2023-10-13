package bookmark

import (
	"database/sql"
	"time"
)

type Bookmark struct {
	Title     string
	Text      string
	URL       string
	IsScraped sql.NullBool
	DateAdded SqlTime
}

type SqlTime time.Time

func (st *SqlTime) Scan(value interface{}) error {
	if value == nil {
		*st = SqlTime(time.Time{})
		return nil
	}

	parsedTime, err := time.Parse(time.RFC3339, value.(string))
	if err != nil {
		return err
	}

	*st = SqlTime(parsedTime)
	return nil
}

func (st SqlTime) String() string {
	t := time.Time(st)
	return t.Format(time.RFC3339)
}
