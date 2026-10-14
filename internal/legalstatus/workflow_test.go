package legalstatus

import (
	"testing"
	"time"
)

func TestDecideStatus(t *testing.T) {
	now := time.Date(2026, time.September, 26, 10, 0, 0, 0, time.UTC)
	base := StatusInput{
		RequestID: "req-42", MatterID: "MAT-204", DeviceID: "intake-kiosk-3",
		DeviceOnline: true, IntakeComplete: true, SignedDocumentDelivered: true,
		Deadline: now.Add(7 * 24 * time.Hour),
	}

	tests := []struct {
		name string
		edit func(*StatusInput)
		want string
	}{
		{name: "missing intake wins", edit: func(in *StatusInput) { in.IntakeComplete = false }, want: StateIntakeRequired},
		{name: "signed delivery pending", edit: func(in *StatusInput) { in.SignedDocumentDelivered = false }, want: StateSignaturePending},
		{name: "deadline at 48 hours", edit: func(in *StatusInput) { in.Deadline = now.Add(48 * time.Hour) }, want: StateDeadlineFollowUp},
		{name: "completed and outside follow-up window", edit: func(*StatusInput) {}, want: StateOnTrack},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := base
			tt.edit(&in)
			got, err := DecideStatus(in, now)
			if err != nil {
				t.Fatal(err)
			}
			if got.State != tt.want {
				t.Fatalf("state = %q, want %q", got.State, tt.want)
			}
		})
	}
}
