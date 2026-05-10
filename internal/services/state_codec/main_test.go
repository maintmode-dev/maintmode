package statecodec

import (
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	code := m.Run()

	os.Exit(code)
}

func initService(t *testing.T) *Service {
	t.Helper()

	return NewService(
		[]byte("test-secret-test-secret-32bytes!"),
		time.Minute,
	)
}
