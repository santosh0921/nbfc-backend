// Package finance implements the Phase 8 genuinely-new pieces the audit
// found missing: vendor/expense management, bank-account balance
// tracking, and the executive financial dashboard. Everything else Phase
// 8 asked to verify (chart of accounts, double-entry ledger, journal
// posting, period controls, maker-checker, disbursement/collection
// accounting) already exists from Phase 4/5 (internal/accounting,
// internal/risk) — this package builds on top of it via
// accounting.Service, never duplicates it.
package finance

import (
	"errors"
	"strings"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
	"github.com/santosh0921/nbfc-backend/internal/security"
)

// CreateVendor — PAN is encrypted at rest using the same AES-256-GCM
// mechanism as customer PAN (internal/security, Phase 1/3) — a vendor's
// PAN is exactly as sensitive as a customer's and gets the same
// treatment, not a lesser one.
func CreateVendor(name, category, pan, gstin, bankAccountMasked, ifsc, createdBy string) (*models.Vendor, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("finance: vendor name is required")
	}
	v := models.Vendor{
		Name: name, Category: category, GSTIN: gstin,
		BankAccountMasked: bankAccountMasked, IFSC: ifsc,
		Status: models.VendorStatusActive, CreatedBy: createdBy,
	}
	if pan != "" {
		encrypted, err := security.Encrypt(pan)
		if err != nil {
			return nil, err
		}
		v.PANEncrypted = encrypted
		if len(pan) >= 4 {
			v.PANLast4 = pan[len(pan)-4:]
		}
	}
	if err := database.DB.Create(&v).Error; err != nil {
		return nil, err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: createdBy,
		Action: "finance.vendor.created", TargetType: "vendor", TargetID: itoa(v.ID),
		Result: audit.ResultSuccess, Details: name,
	})
	return &v, nil
}

func SetVendorStatus(vendorID uint, status, actor string) error {
	if status != models.VendorStatusActive && status != models.VendorStatusInactive {
		return errors.New("finance: invalid vendor status")
	}
	if err := database.DB.Model(&models.Vendor{}).Where("id = ?", vendorID).Update("status", status).Error; err != nil {
		return err
	}
	audit.RecordDetailed(nil, audit.Entry{
		ActorType: "admin", ActorRef: actor,
		Action: "finance.vendor.status_changed", TargetType: "vendor", TargetID: itoa(vendorID),
		Result: audit.ResultSuccess, Details: status,
	})
	return nil
}
