package risk

import (
	"log"
	"time"

	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// SeedDefaultBuckets is idempotent (inserts only missing bucket names) and
// seeds ONLY DelinquencyBucket rows — the example structure item C
// explicitly permits ("do not assume these are the final regulatory
// classifications"). It deliberately does NOT seed any ClassificationRule
// or ProvisioningRule row: those carry actual regulatory/accounting
// consequences (asset classification, provisioning journal entries) and
// item D/G/W are explicit that no threshold or percentage may be invented
// — those tables start and stay EMPTY until the NBFC's compliance
// officer/CA configures and approves real rule rows via the admin
// maker-checker flow (internal/risk/rules_handler.go). Until then,
// ClassifyLoan/CalculateProvision simply find no APPROVED rule and do
// nothing, which is the correct, conservative behavior — not a bug.
func SeedDefaultBuckets() {
	buckets := []struct {
		name             string
		min, max, priority int
	}{
		{"CURRENT", 0, 0, 1},
		{"EARLY_OVERDUE", 1, 30, 2},
		{"DELINQUENT", 31, 60, 3},
		{"HIGH_RISK", 61, 90, 4},
		{"NPA", 91, 999999, 5},
	}
	for _, b := range buckets {
		var existing models.DelinquencyBucket
		if err := database.DB.Where("name = ?", b.name).First(&existing).Error; err == nil {
			continue
		}
		database.DB.Create(&models.DelinquencyBucket{
			Name: b.name, MinDPD: b.min, MaxDPD: b.max, Priority: b.priority,
			Status: models.BucketStatusActive, EffectiveFrom: time.Now(),
			RuleVersion: "bucket-seed-v1-UNAPPROVED-EXAMPLE",
		})
	}
	log.Println("✅ Delinquency buckets seeded/verified (example DPD ranges — NOT validated against any regulatory framework; see the Phase 5 report)")
	log.Println("⚠️  No ClassificationRule/ProvisioningRule rows are seeded — asset classification and provisioning will not produce results for any loan until compliance/CA configures and approves them via the admin Risk Rules screen")
}
