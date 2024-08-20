package utils

import (
	"database/sql"
	"time"
)

func SqlNullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	} else {
		return ""
	}
}

func SqlNullTime(value sql.NullTime) time.Time {
	if value.Valid {
		return value.Time
	} else {
		return time.Time{}
	}
}
