package admin

import (
	"log"

	"github.com/santosh0921/nbfc-backend/internal/database"
)

// perfIndex is one Phase 15 performance index, carrying the justification
// item E requires alongside the DDL rather than in a separate document
// that could drift from it.
type perfIndex struct {
	name string
	ddl  string
	// benefit / writeCost / justification are documentation, kept here so
	// a future maintainer can judge whether the index still earns its
	// place rather than guessing why it exists.
	benefit       string
	writeCost     string
	justification string
}

// Every index below was added because a REAL query pattern in this
// codebase needs it — each was verified by locating the actual queries
// first (counts noted in `justification`), per item E's instruction not
// to blindly index every column. Single-column indexes that GORM model
// tags already create are deliberately NOT duplicated here.
//
// All are created CONCURRENTLY and IF NOT EXISTS: CREATE INDEX takes an
// ACCESS EXCLUSIVE lock on the table by default, which on a 100k+ row
// table in production would block reads and writes for the duration.
// CONCURRENTLY trades a longer build for not locking out traffic — the
// right trade for a migration that runs against a live database (item
// AB).
var perfIndexes = []perfIndex{
	{
		name: "idx_users_created_at",
		ddl:  `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_users_created_at ON users (created_at DESC)`,
		benefit:   "Admin customer list orders by created_at DESC on every page load; without this, paging a 100k-row table re-sorts the whole table per request.",
		writeCost: "One extra B-tree entry per customer registration — negligible against registration volume.",
		justification: "internal/admin/users_list_handler.go ListUsersHandler (the endpoint measured at 12.97s p50 before Phase 15's rewrite).",
	},
	{
		name: "idx_loan_applications_status_created_at",
		ddl:  `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_loan_applications_status_created_at ON loan_applications (status, created_at)`,
		benefit:   "Serves both the status-filtered admin loan list and the 10 analytics queries shaped `status IN (...) AND created_at BETWEEN ...` (growth counts, funnel stages, approval rates).",
		writeCost: "One composite entry per application insert and per status transition. Loan status changes a handful of times over a loan's life, so this is cheap.",
		justification: "10 verified occurrences of the status+created_at range pattern across internal/analytics and internal/loans.",
	},
	{
		name: "idx_emi_installments_due_date",
		ddl:  `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_emi_installments_due_date ON emi_installments (due_date)`,
		benefit:   "DPD bucketing, collection-efficiency denominators, and overdue sweeps all scan by due-date range. emi_installments is the largest transactional table (one row per installment per loan), so a full scan here is the most expensive scan in the system.",
		writeCost: "One entry per installment, written once at schedule generation and rarely updated.",
		justification: "13 verified occurrences of due_date range predicates (internal/analytics DPD/collection code, internal/risk, internal/collections).",
	},
	{
		name: "idx_emi_installments_loan_id_status",
		ddl:  `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_emi_installments_loan_id_status ON emi_installments (loan_id, status)`,
		benefit:   "Serves the `EXISTS (... WHERE loan_id = ? AND status != 'paid')` derivation used for closed-loan and overdue-loan detection, which runs per loan inside portfolio and cohort aggregations.",
		writeCost: "One composite entry per installment; updated when an installment is marked paid (once per installment).",
		justification: "12 verified occurrences of the loan_id + status predicate combination.",
	},
	{
		name: "idx_journal_entries_transaction_date",
		ddl:  `CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_journal_entries_transaction_date ON journal_entries (transaction_date)`,
		benefit:   "Every P&L, revenue, expense, funding-cost, and collections aggregation filters journal entries by transaction_date range. journal_entries grows fastest of any table (multiple rows per financial event).",
		writeCost: "One entry per journal entry. Postings are append-only, so no update churn.",
		justification: "10 verified occurrences of transaction_date BETWEEN across internal/analytics, internal/finance, and internal/accounting.",
	},
}

// CreatePerformanceIndexes applies the Phase 15 indexes.
//
// Deliberately NOT part of admin.Migrate()'s AutoMigrate call: GORM would
// create these with a blocking lock, and AutoMigrate runs on every boot
// where a slow concurrent build would delay startup. This is called once
// after migration, is idempotent (IF NOT EXISTS), and logs rather than
// fatals on failure — an index that fails to build is a performance
// problem, never a reason to refuse to serve traffic.
func CreatePerformanceIndexes() {
	created := 0
	for _, idx := range perfIndexes {
		// CONCURRENTLY cannot run inside a transaction block; GORM's Exec
		// runs outside one here, which is why this is a plain Exec and not
		// part of a migration transaction.
		if err := database.DB.Exec(idx.ddl).Error; err != nil {
			log.Printf("⚠️  performance index %s not created: %v (the application will still work; queries relying on it will be slower)", idx.name, err)
			continue
		}
		created++
	}
	log.Printf("✅ Performance indexes verified (%d/%d present) — see internal/admin/perf_indexes.go for each index's justification", created, len(perfIndexes))
}
