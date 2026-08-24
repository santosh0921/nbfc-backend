package perf_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/santosh0921/nbfc-backend/internal/accounting"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Phase 15 concurrency suite — items M (payment), N (loan), O
// (accounting). The phase brief calls payment concurrency CRITICAL, and
// these are the tests that decide whether "financial transaction load:
// READY" can honestly be claimed.
//
// Unlike the load tests above, these do NOT require PERF=1: correctness
// under concurrency is a permanent invariant, not an opt-in benchmark,
// so they run as part of the normal regression suite. They are cheap
// (a few dozen rows, cleaned up) and use a distinct marker so they can
// never collide with real or perf-seeded data.

const concTestActor = "phase15-concurrency-test"

func uniqueKey(t *testing.T) string {
	return fmt.Sprintf("%s-%s", t.Name(), time.Now().Format("150405.000000000"))
}

func cleanupJournals(t *testing.T, keyPrefix string) {
	t.Helper()
	t.Cleanup(func() {
		var ids []uint
		database.DB.Model(&models.JournalEntry{}).
			Where("idempotency_key LIKE ?", keyPrefix+"%").Pluck("id", &ids)
		if len(ids) > 0 {
			database.DB.Unscoped().Where("journal_id IN ?", ids).Delete(&models.JournalEntryLine{})
			database.DB.Unscoped().Where("id IN ?", ids).Delete(&models.JournalEntry{})
		}
	})
}

// --- Item O: concurrent journal posting must be exactly-once ---
//
// The same business event posted concurrently from N goroutines (the
// real-world shape of a gateway retrying a webhook while the original is
// still in flight) must produce exactly ONE journal entry, not N.
func TestConcurrentIdenticalJournalPostsAreExactlyOnce(t *testing.T) {
	ensureConcurrencyDB(t)

	key := uniqueKey(t)
	cleanupJournals(t, key)

	const goroutines = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	var successes, failures int

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start // release all at once, maximizing real contention
			_, err := accounting.Service.PostJournalEntry(accounting.PostingInput{
				TransactionType: "EMI_PAYMENT",
				ReferenceType:   "payment",
				ReferenceID:     key,
				IdempotencyKey:  key,
				TransactionDate: time.Now(),
				Description:     "Phase 15 concurrency test",
				CreatedBy:       concTestActor,
				Lines: []accounting.LineInput{
					{AccountCode: accounting.AcctBankCash, DebitPaise: 100000, Description: "cash in"},
					{AccountCode: accounting.AcctLoanPrincipalReceivable, CreditPaise: 100000, Description: "principal repaid"},
				},
			})
			mu.Lock()
			if err == nil {
				successes++
			} else {
				failures++
			}
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()

	var count int64
	database.DB.Model(&models.JournalEntry{}).Where("idempotency_key = ?", key).Count(&count)

	if count != 1 {
		t.Fatalf("EXACTLY-ONCE VIOLATED: %d goroutines posting the same idempotency key produced %d journal entries (want exactly 1). "+
			"successes=%d failures=%d", goroutines, count, successes, failures)
	}
	t.Logf("%d concurrent identical posts → exactly 1 journal entry (successes=%d, deduplicated=%d)",
		goroutines, successes, failures)
}

// --- Item O: every posting balances, even under concurrent load ---
//
// Debits must equal credits for every entry created concurrently. A
// partially-committed entry (cash leg without its receivable leg) would
// silently corrupt the ledger.
func TestConcurrentDistinctJournalPostsAllBalance(t *testing.T) {
	ensureConcurrencyDB(t)

	prefix := uniqueKey(t)
	cleanupJournals(t, prefix)

	const goroutines = 24
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			amount := int64(1000 * (n + 1))
			_, _ = accounting.Service.PostJournalEntry(accounting.PostingInput{
				TransactionType: "EMI_PAYMENT",
				ReferenceType:   "payment",
				ReferenceID:     fmt.Sprintf("%s-%d", prefix, n),
				IdempotencyKey:  fmt.Sprintf("%s-%d", prefix, n),
				TransactionDate: time.Now(),
				Description:     "Phase 15 balance-under-load test",
				CreatedBy:       concTestActor,
				Lines: []accounting.LineInput{
					{AccountCode: accounting.AcctBankCash, DebitPaise: amount, Description: "cash in"},
					{AccountCode: accounting.AcctLoanPrincipalReceivable, CreditPaise: amount, Description: "principal repaid"},
				},
			})
		}(i)
	}
	close(start)
	wg.Wait()

	// Verify Debit == Credit for every entry this test created.
	type row struct {
		JournalID uint
		Debits    int64
		Credits   int64
	}
	var rows []row
	database.DB.Raw(`
		SELECT jel.journal_id,
		       COALESCE(SUM(jel.debit_paise),0)  AS debits,
		       COALESCE(SUM(jel.credit_paise),0) AS credits
		FROM journal_entry_lines jel
		JOIN journal_entries je ON je.id = jel.journal_id
		WHERE je.idempotency_key LIKE ?
		GROUP BY jel.journal_id
	`, prefix+"%").Scan(&rows)

	if len(rows) == 0 {
		t.Fatal("no journal entries were created — the test proved nothing")
	}
	for _, r := range rows {
		if r.Debits != r.Credits {
			t.Fatalf("UNBALANCED ENTRY under concurrency: journal %d has debits=%d credits=%d",
				r.JournalID, r.Debits, r.Credits)
		}
	}
	t.Logf("%d concurrently-posted entries, all balanced (debits == credits)", len(rows))
}

// --- Item M: duplicate webhook / duplicate payment protection ---
//
// models.PaymentEvent.GatewayEventID carries a unique index specifically
// so a replayed gateway webhook cannot be processed twice. This proves
// the constraint actually holds under concurrent insertion, not just in
// sequential code.
func TestConcurrentDuplicateGatewayEventRejected(t *testing.T) {
	ensureConcurrencyDB(t)

	eventID := "phase15-evt-" + uniqueKey(t)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("gateway_event_id = ?", eventID).Delete(&models.PaymentEvent{})
	})

	const goroutines = 12
	var wg sync.WaitGroup
	var mu sync.Mutex
	inserted := 0

	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			ev := models.PaymentEvent{
				GatewayEventID: eventID,
				EventType:      "payment.captured",
			}
			if err := database.DB.Create(&ev).Error; err == nil {
				mu.Lock()
				inserted++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	var count int64
	database.DB.Model(&models.PaymentEvent{}).Where("gateway_event_id = ?", eventID).Count(&count)
	if count != 1 {
		t.Fatalf("REPLAY PROTECTION VIOLATED: %d concurrent inserts of the same gateway event id produced %d rows (want exactly 1)",
			goroutines, count)
	}
	t.Logf("%d concurrent duplicate webhook events → exactly 1 stored (%d insert(s) succeeded, rest rejected by unique index)",
		goroutines, inserted)
}

// --- Item N: one disbursement per loan, under concurrency ---
//
// models.Disbursement.LoanID has a unique index so a loan can never be
// disbursed twice. This proves it holds when two operators (or a
// double-clicked button) race.
func TestConcurrentDoubleDisbursementPrevented(t *testing.T) {
	ensureConcurrencyDB(t)

	loan := models.LoanApplication{
		CustomerID: uuidNew(), Category: "Personal Loan", AmountRequested: 50000,
		Status: models.LoanStatusSanctioned, ApplicantName: "Phase15 Concurrency",
	}
	database.DB.Create(&loan)
	t.Cleanup(func() {
		database.DB.Unscoped().Where("loan_id = ?", loan.ID).Delete(&models.Disbursement{})
		database.DB.Unscoped().Delete(&loan)
	})

	const goroutines = 10
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			d := models.Disbursement{
				LoanID: loan.ID, CustomerID: loan.CustomerID.String(),
				AmountPaise: 5000000, Status: models.DisbursementStatusSuccess,
				ProviderReference: fmt.Sprintf("phase15-disb-%d-%d", loan.ID, n),
			}
			_ = database.DB.Create(&d).Error
		}(i)
	}
	close(start)
	wg.Wait()

	var count int64
	database.DB.Model(&models.Disbursement{}).Where("loan_id = ?", loan.ID).Count(&count)
	if count != 1 {
		t.Fatalf("DOUBLE DISBURSEMENT: %d concurrent attempts on loan %d produced %d disbursement rows (want exactly 1)",
			goroutines, loan.ID, count)
	}
	t.Logf("%d concurrent disbursement attempts on one loan → exactly 1 row", goroutines)
}

// ensureConcurrencyDB connects without requiring the opt-in PERF flag —
// these correctness tests always run.
func ensureConcurrencyDB(t *testing.T) {
	t.Helper()
	if database.DB == nil {
		ensureApp(t)
	}
}

func uuidNew() uuid.UUID { return uuid.New() }

// --- Journal numbering must survive deletion (Phase 15 regression) ---
//
// Found during Phase 15 regression: nextJournalNumber derived the next
// sequence value from COUNT(*)+1, so deleting ANY journal entry moved the
// counter backwards and the next posting collided with an existing
// journal_number. The unique index then rejected it — and kept rejecting
// every subsequent posting, i.e. a permanent outage of all financial
// posting triggered by a single row deletion.
//
// This test posts, deletes, and posts again: with a COUNT-based sequence
// the second post fails; with the MAX-based sequence it succeeds and
// simply leaves a gap, which is the correct behaviour for an audit
// sequence.
func TestJournalNumberingSurvivesDeletion(t *testing.T) {
	ensureConcurrencyDB(t)

	prefix := uniqueKey(t)
	cleanupJournals(t, prefix)

	post := func(n int) (*models.JournalEntry, error) {
		return accounting.Service.PostJournalEntry(accounting.PostingInput{
			TransactionType: "EMI_PAYMENT",
			ReferenceType:   "payment",
			ReferenceID:     fmt.Sprintf("%s-%d", prefix, n),
			IdempotencyKey:  fmt.Sprintf("%s-%d", prefix, n),
			TransactionDate: time.Now(),
			Description:     "Phase 15 journal-numbering regression",
			CreatedBy:       concTestActor,
			Lines: []accounting.LineInput{
				{AccountCode: accounting.AcctBankCash, DebitPaise: 5000, Description: "cash in"},
				{AccountCode: accounting.AcctLoanPrincipalReceivable, CreditPaise: 5000, Description: "principal repaid"},
			},
		})
	}

	first, err := post(1)
	if err != nil {
		t.Fatalf("first posting failed: %v", err)
	}

	// Remove it, exactly as a data correction or cleanup would.
	database.DB.Unscoped().Where("journal_id = ?", first.ID).Delete(&models.JournalEntryLine{})
	database.DB.Unscoped().Where("id = ?", first.ID).Delete(&models.JournalEntry{})

	second, err := post(2)
	if err != nil {
		t.Fatalf("posting after a deletion must still succeed — a count-derived journal sequence would collide here and permanently break all posting: %v", err)
	}
	if second.JournalNumber == first.JournalNumber {
		t.Fatalf("journal number %q was REUSED after deletion — an audit sequence must never reuse a number", second.JournalNumber)
	}
	t.Logf("posting survived deletion: %s then %s (gap is correct; reuse would not be)",
		first.JournalNumber, second.JournalNumber)
}
