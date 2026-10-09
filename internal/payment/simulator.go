package payment

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

// Payment modes accepted by Simulate and by POST /api/v1/checkout.
const (
	ModeApprove = "approve"
	ModeDecline = "decline"
	ModeFail    = "fail"
)

// ErrDeclined is returned when the (simulated) acquirer refuses the charge.
// ErrGateway is returned when the (simulated) acquirer itself errors out.
var (
	ErrDeclined = errors.New("payment declined by acquirer")
	ErrGateway  = errors.New("payment gateway error")
)

// Simulate mimics an acquirer authorization without any network I/O:
//   - approve → (true, authCode, nil)
//   - decline → (false, "", ErrDeclined)
//   - fail    → (false, "", ErrGateway)
//
// Any other mode returns an error so callers can map it to 400.
func Simulate(mode string) (approved bool, authCode string, err error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeApprove:
		return true, newAuthCode(), nil
	case ModeDecline:
		return false, "", ErrDeclined
	case ModeFail:
		return false, "", ErrGateway
	default:
		return false, "", fmt.Errorf("unknown payment mode %q: want approve|decline|fail", mode)
	}
}

// ValidMode reports whether mode is one of the accepted payment modes
// (empty means "default", which the checkout treats as approve).
func ValidMode(mode string) bool {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", ModeApprove, ModeDecline, ModeFail:
		return true
	default:
		return false
	}
}

// Normalize returns the effective mode, mapping "" to approve.
func Normalize(mode string) string {
	if strings.TrimSpace(mode) == "" {
		return ModeApprove
	}
	return strings.ToLower(strings.TrimSpace(mode))
}

func newAuthCode() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "AUTH-UNKNOWN"
	}
	return "AUTH-" + strings.ToUpper(hex.EncodeToString(b[:]))
}
