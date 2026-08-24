package tax

import "log"

// SeedNothing is called from main.go purely to document, at startup, that
// this package deliberately seeds ZERO TDSRule rows. Every prior phase
// that touched a regulatory figure (Phase 5's classification/provisioning
// rules, Phase 6's regulatory report definitions) followed the same
// pattern for the same reason: item Y/Z require every rate/threshold/
// section/form-name to be verified against an official source and
// approved by a CA/tax professional before use — this codebase cannot
// discharge that requirement, so it doesn't pretend to. Until an admin
// drafts and approves at least one TDSRule via
// POST /admin/tax/rules (+.../approve), CalculateTDS will correctly
// return ErrNoApplicableRule for every payment, which is the safe,
// intended default — not a missing feature.
func SeedNothing() {
	log.Println("⚠️  No TDSRule rows are seeded — TDS will not be calculated for any payment until compliance/CA configures and approves rules via the admin Tax Rules screen (see internal/tax's package doc comment)")
}
