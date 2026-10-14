package legalstatus

import (
	"errors"
	"time"
)

const (
	StateIntakeRequired   = "intake_required"
	StateSignaturePending = "signature_pending"
	StateDeadlineFollowUp = "deadline_follow_up"
	StateOnTrack          = "on_track"
)

type StatusInput struct {
	RequestID               string    `json:"request_id"`
	MatterID                string    `json:"matter_id"`
	DeviceID                string    `json:"device_id"`
	DeviceOnline            bool      `json:"device_online"`
	IntakeComplete          bool      `json:"intake_complete"`
	SignedDocumentDelivered bool      `json:"signed_document_delivered"`
	Deadline                time.Time `json:"deadline"`
}

type Decision struct {
	MatterID     string    `json:"matter_id"`
	DeviceID     string    `json:"device_id"`
	DeviceOnline bool      `json:"device_online"`
	State        string    `json:"state"`
	Deadline     time.Time `json:"deadline"`
}

func DecideStatus(in StatusInput, now time.Time) (Decision, error) {
	if in.RequestID == "" || in.MatterID == "" || in.DeviceID == "" || in.Deadline.IsZero() {
		return Decision{}, errors.New("request_id, matter_id, device_id, and deadline are required")
	}

	state := StateOnTrack
	switch {
	case !in.IntakeComplete:
		state = StateIntakeRequired
	case !in.SignedDocumentDelivered:
		state = StateSignaturePending
	case !in.Deadline.After(now.Add(48 * time.Hour)):
		state = StateDeadlineFollowUp
	}

	return Decision{
		MatterID:     in.MatterID,
		DeviceID:     in.DeviceID,
		DeviceOnline: in.DeviceOnline,
		State:        state,
		Deadline:     in.Deadline,
	}, nil
}
