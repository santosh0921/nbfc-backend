package compliance

import "log"

// SeedNothing documents, at startup, that this package seeds ZERO
// ComplianceRequirement, NBFCProfile field values, RegulatoryReturnTracking,
// or Policy rows. Item AB requires every regulatory requirement to be
// recorded with its official source and verified before use; item A
// requires the NBFC's actual category/layer to be confirmed, never
// guessed. Neither can be discharged by this codebase — an admin/
// compliance officer must configure the NBFCProfile (then explicitly
// ConfirmProfile) and draft+activate real ComplianceRequirement rows via
// the admin Compliance screens before this module reflects anything
// about this NBFC's actual regulatory obligations. Until then, an empty
// compliance register is the correct, honest state — not a bug.
func SeedNothing() {
	log.Println("⚠️  No compliance requirements/NBFC profile fields are seeded — configure and confirm the NBFC profile and activate real, source-verified requirements via the admin Compliance screens before this module reflects any actual regulatory obligation")
}
