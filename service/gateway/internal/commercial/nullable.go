package commercial

import (
	"database/sql"
	"strings"
	"time"
)

func NullableTimeToAny(value sql.NullTime) any {
	if !value.Valid {
		return nil
	}
	return value.Time.Format(time.RFC3339)
}

func NullableInt64ToAny(value sql.NullInt64) any {
	if !value.Valid {
		return nil
	}
	return value.Int64
}

func NullableStringToString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}
