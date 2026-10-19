package admin_test

import (
	"net/http"
	"testing"

	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Phase 4 — AdminDecisionHandler ("reject") state whitelist regression
// tests. Before this fix the handler rejected a loan from ANY status,
// including disbursed (real money already moved) and rejected itself
// (already terminal). See internal/loans/admin_handler.go's
// canDirectlyReject.

func TestRejectValidFromVerified(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_reject_valid_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusVerified)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "does not qualify"})
	if w.Code != http.StatusOK {
		t.Fatalf("a legitimate pre-sanction rejection must succeed, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRejectInvalidFromSanctioned(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_reject_sanctioned_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusSanctioned)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "attempt"})
	if w.Code != http.StatusConflict {
		t.Fatalf("rejecting an already-sanctioned loan via this endpoint must be refused, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRejectDisbursedLoanBlocked(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_reject_disbursed_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusDisbursed)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "attempt"})
	if w.Code != http.StatusConflict {
		t.Fatalf("a disbursed loan (real money already moved) must never become 'rejected', got %d: %s", w.Code, w.Body.String())
	}
}

func TestRejectAlreadyRejectedBlocked(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_reject_terminal_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusRejected)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "duplicate"})
	if w.Code != http.StatusConflict {
		t.Fatalf("rejected is terminal — re-rejecting must be refused, not silently accepted, got %d: %s", w.Code, w.Body.String())
	}
}

func TestRejectDuplicateRequestFromSubmitted(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_reject_dup_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusSubmitted)
	w1 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "first"})
	if w1.Code != http.StatusOK {
		t.Fatalf("first rejection should succeed, got %d: %s", w1.Code, w1.Body.String())
	}

	w2 := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "duplicate request"})
	if w2.Code != http.StatusConflict {
		t.Fatalf("a repeated reject request on an already-rejected loan must be refused, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestRejectValidFromPendingClaim(t *testing.T) {
	r := setupTestApp(t)
	username := "sectest_reject_claim_" + t.Name()
	_, password := createTestAdmin(t, username, models.AdminRoleSuperAdmin)
	token, _ := loginAs(t, r, username, password)

	loan := createTestLoan(t, models.LoanStatusPendingClaim)
	w := doJSON(r, "POST", "/admin/loans/"+itoaTest(loan.ID)+"/decision", token,
		map[string]interface{}{"action": "reject", "remarks": "does not qualify after re-verification"})
	if w.Code != http.StatusOK {
		t.Fatalf("pending_claim is a legitimate pre-sanction state and must remain rejectable, got %d: %s", w.Code, w.Body.String())
	}
}
