// Package accounting implements the Phase 4 double-entry general ledger
// foundation: Chart of Accounts, JournalEntry/JournalEntryLine posting,
// accounting periods, and the report queries (Trial Balance, P&L, Balance
// Sheet) built directly from posted journal lines — never from loan
// application tables directly (per the Phase 4 instruction).
//
// AccountingService.PostJournalEntry is the ONLY way any journal is
// created in this codebase — no handler writes JournalEntry/
// JournalEntryLine rows directly. Every posting is atomic (one DB
// transaction), balance-validated (total debits == total credits) before
// commit, and deduplicated by IdempotencyKey so the same business event
// (one Disbursement, one Payment, one accrual period) can never post
// twice.
//
// This package does NOT implement NPA/IRAC classification (Phase 5) — see
// models.InterestAccrual.ExtensionPhase5 and the PROV_NPA/EXP_NPA_PROV
// chart-of-accounts entries seeded in chart.go for the extension points
// left in place for that phase.
package accounting

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

var (
	ErrUnbalancedEntry   = errors.New("accounting: total debits must equal total credits")
	ErrEmptyEntry        = errors.New("accounting: a journal entry must have at least two lines")
	ErrPeriodNotOpen     = errors.New("accounting: the accounting period for this transaction date is not open for posting")
	ErrDuplicatePosting  = errors.New("accounting: this business event has already been posted (idempotency key already used)")
	ErrUnknownAccount    = errors.New("accounting: unknown or inactive account code")
	ErrJournalNotFound   = errors.New("accounting: journal entry not found")
	ErrAlreadyReversed   = errors.New("accounting: this journal entry has already been reversed")
	ErrCannotReversePosted = errors.New("accounting: only a POSTED journal entry can be reversed")
)

// LineInput is one leg of a journal entry the caller wants posted — exactly
// one of DebitPaise/CreditPaise must be nonzero.
type LineInput struct {
	AccountCode string
	DebitPaise  int64
	CreditPaise int64
	Description string
}

// PostingInput is everything AccountingService.PostJournalEntry needs to
// atomically create one balanced journal entry.
type PostingInput struct {
	TransactionType string
	ReferenceType   string
	ReferenceID     string
	IdempotencyKey  string // if empty, defaults to ReferenceType+":"+ReferenceID+":"+TransactionType
	TransactionDate time.Time
	Description     string
	CreatedBy       string
	ApprovedBy      string
	Lines           []LineInput
}

// toPaise converts a rupee float (as stored on legacy loan/EMI fields) into
// integer paise using round-half-up, the same convention as
// internal/loans' roundToPaise — kept local to this package rather than
// exported cross-package coupling.
func toPaise(rupees float64) int64 {
	return int64(math.Round(rupees * 100))
}

// PostJournalEntry validates, then atomically posts, one balanced journal
// entry. It is the single choke point every business-event posting
// function in this package (postings.go) funnels through.
func (s *service) PostJournalEntry(input PostingInput) (*models.JournalEntry, error) {
	if len(input.Lines) < 2 {
		return nil, ErrEmptyEntry
	}
	var totalDebit, totalCredit int64
	for _, l := range input.Lines {
		if (l.DebitPaise != 0) == (l.CreditPaise != 0) {
			return nil, errors.New("accounting: each line must have exactly one of debit/credit set")
		}
		totalDebit += l.DebitPaise
		totalCredit += l.CreditPaise
	}
	if totalDebit != totalCredit {
		return nil, ErrUnbalancedEntry
	}
	if totalDebit == 0 {
		return nil, ErrEmptyEntry
	}

	idempotencyKey := input.IdempotencyKey
	if idempotencyKey == "" {
		idempotencyKey = input.ReferenceType + ":" + input.ReferenceID + ":" + input.TransactionType
	}

	// Idempotency check outside the transaction is a best-effort fast
	// path; the unique index on IdempotencyKey is the real guarantee
	// against a race (two concurrent posts for the same event) — a
	// constraint violation on insert is treated as ErrDuplicatePosting
	// below, not a generic DB error.
	var existing models.JournalEntry
	if err := database.DB.Where("idempotency_key = ?", idempotencyKey).First(&existing).Error; err == nil {
		return nil, ErrDuplicatePosting
	}

	txDate := input.TransactionDate
	if txDate.IsZero() {
		txDate = time.Now()
	}

	var result models.JournalEntry
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		period, err := getOrCreatePeriodForDate(tx, txDate)
		if err != nil {
			return err
		}
		if period.Status == models.PeriodStatusClosed {
			return ErrPeriodNotOpen
		}

		accountIDs := make(map[string]uint, len(input.Lines))
		for _, l := range input.Lines {
			if _, ok := accountIDs[l.AccountCode]; ok {
				continue
			}
			var acct models.LedgerAccount
			if err := tx.Where("account_code = ? AND status = ?", l.AccountCode, models.AccountStatusActive).First(&acct).Error; err != nil {
				return fmt.Errorf("%w: %s", ErrUnknownAccount, l.AccountCode)
			}
			accountIDs[l.AccountCode] = acct.ID
		}

		journalNumber, err := nextJournalNumber(tx)
		if err != nil {
			return err
		}

		entry := models.JournalEntry{
			JournalNumber:   journalNumber,
			TransactionType: input.TransactionType,
			ReferenceType:   input.ReferenceType,
			ReferenceID:     input.ReferenceID,
			IdempotencyKey:  idempotencyKey,
			TransactionDate: txDate,
			PostingDate:     time.Now(),
			PeriodID:        period.ID,
			Description:     input.Description,
			Status:          models.JournalStatusPosted,
			CreatedBy:       input.CreatedBy,
			ApprovedBy:      input.ApprovedBy,
		}
		if err := tx.Create(&entry).Error; err != nil {
			// A unique-constraint violation on idempotency_key here means a
			// concurrent request won the race — surface it as the same
			// duplicate error the caller already knows how to handle.
			return ErrDuplicatePosting
		}

		lines := make([]models.JournalEntryLine, 0, len(input.Lines))
		for _, l := range input.Lines {
			lines = append(lines, models.JournalEntryLine{
				JournalID:   entry.ID,
				AccountID:   accountIDs[l.AccountCode],
				DebitPaise:  l.DebitPaise,
				CreditPaise: l.CreditPaise,
				Description: l.Description,
			})
		}
		if err := tx.Create(&lines).Error; err != nil {
			return err
		}
		entry.Lines = lines
		result = entry
		return nil
	})
	if err != nil {
		return nil, err
	}

	audit.RecordDetailed(nil, audit.Entry{
		ActorType: actorTypeFor(input.CreatedBy), ActorRef: input.CreatedBy,
		Action: "journal.posted", TargetType: "journal_entry", TargetID: strconv.Itoa(int(result.ID)),
		Result: audit.ResultSuccess,
		Details: result.JournalNumber + " " + input.TransactionType + " ref=" + input.ReferenceType + ":" + input.ReferenceID,
	})

	return &result, nil
}

func actorTypeFor(createdBy string) string {
	if createdBy == "system" || createdBy == "" {
		return "system"
	}
	return "admin"
}

// nextJournalNumber generates a sequential, year-scoped journal number
// under the caller's existing transaction (so concurrent posts serialize
// via the row lock on the max-number row rather than colliding). Format:
// JE-<year>-<6-digit-seq>.
func nextJournalNumber(tx *gorm.DB) (string, error) {
	year := time.Now().Year()
	prefix := fmt.Sprintf("JE-%d-", year)

	// Postgres rejects "FOR UPDATE" combined with an aggregate (COUNT), so
	// the row lock this needs to serialize concurrent posts against is
	// taken via a transaction-scoped advisory lock keyed on the year
	// prefix instead — released automatically at commit/rollback.
	if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", prefix).Error; err != nil {
		return "", err
	}
	// PHASE 15 FIX — this previously used COUNT(*)+1.
	//
	// A COUNT-derived sequence moves BACKWARDS the moment any journal
	// entry is removed (a data correction, an erasure request, a
	// rolled-back migration). The next generated number then collides
	// with a journal_number that still exists, and the unique index
	// rejects it — making every subsequent posting fail permanently, not
	// just once. A single row deletion becomes a hard outage of all
	// financial posting.
	//
	// MAX(suffix)+1 fixes the outage but still REUSES a number when the
	// highest entry is the one deleted — unacceptable for an audit
	// sequence, where a number must identify exactly one entry for all
	// time.
	//
	// A Postgres sequence is the correct primitive: it is non-
	// transactional (a rolled-back posting consumes its number rather
	// than handing it to the next writer), never decreases, and is
	// concurrency-safe on its own. Deletions leave GAPS, which is the
	// right behaviour — a missing number is evidence something was
	// removed, whereas a reused number silently rewrites history.
	//
	// The sequence is created per year, lazily, starting above whatever
	// numbers already exist so it is safe to introduce on a database
	// that already holds journal entries.
	seqName := fmt.Sprintf("journal_number_%d", year)
	if err := tx.Exec(`
		DO $$
		DECLARE existing_max bigint;
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_class WHERE relname = `+quoteLiteral(seqName)+` AND relkind = 'S'
			) THEN
				SELECT COALESCE(MAX(NULLIF(regexp_replace(journal_number, '^.*-', ''), '')::bigint), 0)
				INTO existing_max
				FROM journal_entries
				WHERE journal_number LIKE `+quoteLiteral(prefix+"%")+`;
				EXECUTE format('CREATE SEQUENCE %I START WITH %s', `+quoteLiteral(seqName)+`, existing_max + 1);
			END IF;
		END $$;
	`).Error; err != nil {
		return "", err
	}

	var next int64
	if err := tx.Raw(`SELECT nextval(?)`, seqName).Scan(&next).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s%06d", prefix, next), nil
}

// quoteLiteral renders a Go string as a SQL string literal for embedding
// in the DO block above, which cannot take bind parameters. Both call
// sites pass values this function itself constructs from an integer year
// (never user input), but the escaping is applied anyway so the helper
// stays safe if it is ever reused with a different source.
func quoteLiteral(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// getOrCreatePeriodForDate returns the OPEN-by-default calendar-month
// accounting period covering date, creating it if this is the first
// posting ever attempted for that month. Periods are never auto-created
// as CLOSED — only an explicit admin action closes a period (item M).
func getOrCreatePeriodForDate(tx *gorm.DB, date time.Time) (*models.AccountingPeriod, error) {
	code := date.Format("2006-01")
	var period models.AccountingPeriod
	err := tx.Where("code = ?", code).First(&period).Error
	if err == nil {
		return &period, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	start := time.Date(date.Year(), date.Month(), 1, 0, 0, 0, 0, date.Location())
	end := start.AddDate(0, 1, 0).Add(-time.Second)
	period = models.AccountingPeriod{
		Code: code, StartDate: start, EndDate: end, Status: models.PeriodStatusOpen,
	}
	if err := tx.Create(&period).Error; err != nil {
		// Another concurrent posting created the same period first — read
		// it back rather than treating this as a fatal error.
		if lookupErr := tx.Where("code = ?", code).First(&period).Error; lookupErr == nil {
			return &period, nil
		}
		return nil, err
	}
	return &period, nil
}

// service is an unexported placeholder receiver — every exported function
// in this package is a package-level func, not a method on a
// heap-allocated struct, since there is no per-request state to carry.
// PostJournalEntry is defined as a method on it purely so its doc comment
// reads naturally as "AccountingService.PostJournalEntry" per the Phase 4
// brief; Service is the actual exported entry point callers use.
type service struct{}

// Service is the package's single AccountingService instance — stateless,
// safe for concurrent use (all real state lives in the DB, guarded by
// transactions/row locks).
var Service = &service{}
