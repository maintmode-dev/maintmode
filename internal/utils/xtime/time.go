package xtime

import "time"

func UTCNow() time.Time {
	return time.Now().UTC()
}

func StartOfTheDay() time.Time {
	now := UTCNow()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func EndOfTheDay() time.Time {
	now := UTCNow()
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999, time.UTC)
}

func InUTC(t time.Time) time.Time {
	return t.In(time.UTC)
}
