package domain

import "time"

type Deviation struct {
	ID                  string            `json:"id"`
	BatchID             string            `json:"batchId"`
	LoadTestID          string            `json:"loadTestId"`
	RiggingPointID      string            `json:"riggingPointId"`
	Severity            DeviationSeverity `json:"severity"`
	Symptom             string            `json:"symptom"`
	RequiredAction      string            `json:"requiredAction"`
	Assignee            string            `json:"assignee"`
	AssigneeConfirmed   bool              `json:"assigneeConfirmed"`
	ConfirmedBy         string            `json:"confirmedBy,omitempty"`
	ConfirmedAt         *time.Time        `json:"confirmedAt,omitempty"`
	RemediationEvidence string            `json:"remediationEvidence,omitempty"`
	Status              DeviationStatus   `json:"status"`
	ClosedByTestID      string            `json:"closedByTestId,omitempty"`
	ClosedAt            *time.Time        `json:"closedAt,omitempty"`
}

func (d Deviation) ValidateNew() error {
	if d.Severity != SeverityMinor && d.Severity != SeverityMajor && d.Severity != SeverityCritical {
		return Invalid("severity", "偏差等级必须为 minor、major 或 critical")
	}
	if err := validateRequiredText("symptom", d.Symptom, 200); err != nil {
		return err
	}
	if err := validateRequiredText("requiredAction", d.RequiredAction, 200); err != nil {
		return err
	}
	if err := validateRequiredText("assignee", d.Assignee, 60); err != nil {
		return err
	}
	if d.AssigneeConfirmed {
		if err := validateRequiredText("confirmedBy", d.ConfirmedBy, 60); err != nil {
			return err
		}
	}
	return nil
}

func (d *Deviation) ConfirmResponsibility(by string, now time.Time) error {
	if d.Status == DeviationClosed {
		return ErrStateConflict
	}
	if err := validateRequiredText("confirmedBy", by, 60); err != nil {
		return err
	}
	d.AssigneeConfirmed = true
	d.ConfirmedBy = by
	t := now.UTC()
	d.ConfirmedAt = &t
	return nil
}

func (d *Deviation) UpgradeSeverity(next DeviationSeverity) error {
	if d.Status == DeviationClosed {
		return ErrStateConflict
	}
	if next != SeverityMinor && next != SeverityMajor && next != SeverityCritical {
		return Invalid("severity", "偏差等级必须为 minor、major 或 critical")
	}
	rank := func(s DeviationSeverity) int {
		switch s {
		case SeverityMinor:
			return 1
		case SeverityMajor:
			return 2
		case SeverityCritical:
			return 3
		}
		return 0
	}
	if rank(next) <= rank(d.Severity) {
		return Invalid("severity", "open 或已整改偏差只能上调等级")
	}
	d.Severity = next
	return nil
}
