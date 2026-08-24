// Evidence repository + version control (items H/I). Reuses
// internal/storage's existing AES-256-GCM encrypted provider (Phase 3) —
// the exact same mechanism KYC documents use — rather than inventing a
// second file-storage path. Never publicly accessible: StorageReference
// only resolves through storage.Active().Retrieve, gated by the same
// admin authentication/RBAC every other admin route requires.
package compliance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/storage"
)

var ErrNoStorageProvider = errors.New("compliance: no encrypted storage provider is configured — cannot securely store evidence (see DOCUMENT_ENCRYPTION_KEY)")

// UploadEvidence — item H's full field set. decoded is the raw file
// bytes (already decoded from whatever transport encoding the handler
// used); this function never accepts a bare public URL as evidence.
func UploadEvidence(requirementID, calendarItemID *uint, title, category string, decoded []byte, mimeType string, uploadedByAdminID uint, uploadedBy string) (*models.ComplianceEvidence, error) {
	if title == "" {
		return nil, errors.New("compliance: evidence title is required")
	}
	provider := storage.Active()
	if provider == nil {
		return nil, ErrNoStorageProvider
	}
	ref, err := provider.Store(uuid.Nil, uuid.New(), decoded)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(decoded)

	ev := models.ComplianceEvidence{
		RequirementID: requirementID, CalendarItemID: calendarItemID, Title: title, Category: category,
		StorageReference: ref, Hash: hex.EncodeToString(sum[:]), MimeType: mimeType, SizeBytes: len(decoded),
		Version: 1, UploadedByAdminID: uploadedByAdminID, UploadedBy: uploadedBy, UploadedAt: time.Now(),
		Status: models.EvidenceStatusActive,
	}
	if err := database.DB.Create(&ev).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: uploadedByAdminID, ActorRef: uploadedBy,
		Action: "compliance.evidence.uploaded", TargetType: "compliance_evidence", TargetID: itoa(ev.ID),
		Result: audit.ResultSuccess, Details: title,
	})
	return &ev, nil
}

// UploadNewVersion (item I) — never overwrites the previous version's
// row; creates a new ComplianceEvidence row linked via
// PreviousVersionID, and marks the old one SUPERSEDED (still fully
// retrievable, never deleted).
func UploadNewVersion(previousID uint, decoded []byte, mimeType, reason string, uploadedByAdminID uint, uploadedBy string) (*models.ComplianceEvidence, error) {
	var previous models.ComplianceEvidence
	if err := database.DB.First(&previous, previousID).Error; err != nil {
		return nil, err
	}
	provider := storage.Active()
	if provider == nil {
		return nil, ErrNoStorageProvider
	}
	ref, err := provider.Store(uuid.Nil, uuid.New(), decoded)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(decoded)

	newVersion := models.ComplianceEvidence{
		RequirementID: previous.RequirementID, CalendarItemID: previous.CalendarItemID,
		Title: previous.Title, Category: previous.Category,
		StorageReference: ref, Hash: hex.EncodeToString(sum[:]), MimeType: mimeType, SizeBytes: len(decoded),
		Version: previous.Version + 1, PreviousVersionID: &previousID,
		UploadedByAdminID: uploadedByAdminID, UploadedBy: uploadedBy, UploadedAt: time.Now(),
		Status: models.EvidenceStatusActive, Notes: reason,
	}
	if err := database.DB.Create(&newVersion).Error; err != nil {
		return nil, err
	}
	database.DB.Model(&previous).Update("status", models.EvidenceStatusSuperseded)

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorID: uploadedByAdminID, ActorRef: uploadedBy,
		Action: "compliance.evidence.new_version", TargetType: "compliance_evidence", TargetID: itoa(newVersion.ID),
		Result: audit.ResultSuccess, Reason: reason, Details: "supersedes v" + strconv.Itoa(previous.Version) + " (evidence #" + itoa(previousID) + ")",
	})
	return &newVersion, nil
}

// RetrieveEvidence — the only read path; every call is auditable at the
// handler layer (download audit, same pattern as Phase 3 documents).
func RetrieveEvidence(evidenceID uint) (*models.ComplianceEvidence, []byte, error) {
	var ev models.ComplianceEvidence
	if err := database.DB.First(&ev, evidenceID).Error; err != nil {
		return nil, nil, err
	}
	provider := storage.Active()
	if provider == nil {
		return &ev, nil, ErrNoStorageProvider
	}
	data, err := provider.Retrieve(ev.StorageReference)
	if err != nil {
		return &ev, nil, err
	}
	return &ev, data, nil
}
