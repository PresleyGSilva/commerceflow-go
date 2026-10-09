package payment

import (
	"errors"
	"strings"
	"testing"
)

func TestSimulateApprove(t *testing.T) {
	for _, mode := range []string{"approve", "APPROVE", " Approve ", ""} {
		ok, code, err := Simulate(mode)
		if err != nil {
			t.Fatalf("Simulate(%q) err = %v", mode, err)
		}
		if !ok {
			t.Fatalf("Simulate(%q) approved = false, want true", mode)
		}
		if !strings.HasPrefix(code, "AUTH-") || len(code) < 10 {
			t.Fatalf("Simulate(%q) authCode = %q, want AUTH- prefixed", mode, code)
		}
	}
	// Auth codes must be unique per authorization.
	_, a, _ := Simulate("approve")
	_, b, _ := Simulate("approve")
	if a == b {
		t.Fatal("auth codes are not unique")
	}
}

func TestSimulateDecline(t *testing.T) {
	ok, code, err := Simulate("decline")
	if !errors.Is(err, ErrDeclined) {
		t.Fatalf("err = %v, want ErrDeclined", err)
	}
	if ok {
		t.Fatal("approved = true, want false")
	}
	if code != "" {
		t.Fatalf("authCode = %q, want empty", code)
	}
}

func TestSimulateFail(t *testing.T) {
	ok, code, err := Simulate("fail")
	if !errors.Is(err, ErrGateway) {
		t.Fatalf("err = %v, want ErrGateway", err)
	}
	if ok {
		t.Fatal("approved = true, want false")
	}
	if code != "" {
		t.Fatalf("authCode = %q, want empty", code)
	}
}

func TestSimulateUnknownMode(t *testing.T) {
	if _, _, err := Simulate("pix"); err == nil {
		t.Fatal("expected error for unknown mode")
	}
	if ValidMode("pix") {
		t.Fatal("ValidMode(pix) = true, want false")
	}
	for _, m := range []string{"", "approve", "decline", "fail"} {
		if !ValidMode(m) {
			t.Fatalf("ValidMode(%q) = false, want true", m)
		}
	}
	if Normalize("") != ModeApprove {
		t.Fatal("Normalize(\"\") should default to approve")
	}
}
