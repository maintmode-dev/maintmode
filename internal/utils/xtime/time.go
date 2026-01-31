package xtime

import "time"

func UTCNow() time.Time {
	return time.Now().UTC()
}

func StartOfTheCurrentDay() time.Time {
	now := UTCNow()
	return StartOfTheDay(now)
}

func EndOfTheCurrentDay() time.Time {
	now := UTCNow()
	return EndOfTheDay(now)
}

func EndOfTheDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999, time.UTC)
}

func StartOfTheDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
