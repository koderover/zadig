package service

import (
	"net/http"
	"testing"
)

func TestCanManageReleasePlanEditingSession(t *testing.T) {
	session := &ReleasePlanEditingSession{
		SessionID: "session-1",
		UserID:    "owner",
	}

	if !canManageReleasePlanEditingSession(session, "owner", false) {
		t.Fatalf("expected session owner to manage editing session")
	}
	if canManageReleasePlanEditingSession(session, "viewer", false) {
		t.Fatalf("expected non-owner to be denied")
	}
	if !canManageReleasePlanEditingSession(session, "viewer", true) {
		t.Fatalf("expected system admin to manage editing session")
	}
	if canManageReleasePlanEditingSession(nil, "owner", false) {
		t.Fatalf("expected nil session to be denied")
	}
}

func TestCheckReleasePlanCollaborationOrigin(t *testing.T) {
	t.Run("allow empty origin", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://zadig.example.com", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req.Host = "zadig.example.com"

		if !checkReleasePlanCollaborationOrigin(req) {
			t.Fatalf("expected empty origin to be allowed")
		}
	})

	t.Run("allow same host", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://internal", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req.Host = "zadig.example.com"
		req.Header.Set("Origin", "https://zadig.example.com")

		if !checkReleasePlanCollaborationOrigin(req) {
			t.Fatalf("expected same origin host to be allowed")
		}
	})

	t.Run("allow forwarded host", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://internal", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req.Host = "aslan:25000"
		req.Header.Set("X-Forwarded-Host", "zadig.example.com")
		req.Header.Set("Origin", "https://zadig.example.com")

		if !checkReleasePlanCollaborationOrigin(req) {
			t.Fatalf("expected forwarded host to be honored")
		}
	})

	t.Run("reject cross origin host", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, "http://zadig.example.com", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		req.Host = "zadig.example.com"
		req.Header.Set("Origin", "https://evil.example.com")

		if checkReleasePlanCollaborationOrigin(req) {
			t.Fatalf("expected cross origin host to be rejected")
		}
	})
}
