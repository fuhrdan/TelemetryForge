package health

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeChecker struct {
	err error
}

func (checker fakeChecker) Ready(context.Context) error {
	return checker.err
}

func TestReadyWhenDependenciesAreReady(t *testing.T) {
	server := New(":0", slog.New(slog.NewTextHandler(io.Discard, nil)), map[string]Checker{
		"kafka":    fakeChecker{},
		"database": fakeChecker{},
	})

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, response.Code)
	}
}

func TestNotReadyNamesFailedDependency(t *testing.T) {
	server := New(":0", slog.New(slog.NewTextHandler(io.Discard, nil)), map[string]Checker{
		"database": fakeChecker{err: errors.New("down")},
	})

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()
	server.http.Handler.ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected %d, got %d", http.StatusServiceUnavailable, response.Code)
	}
}
