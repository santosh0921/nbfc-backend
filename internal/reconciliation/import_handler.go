package reconciliation

import (
	"encoding/csv"
	"errors"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/santosh0921/nbfc-backend/internal/audit"
	"github.com/santosh0921/nbfc-backend/internal/database"
	"github.com/santosh0921/nbfc-backend/internal/models"
)

// Expected CSV header (case-insensitive, any order):
// date,valueDate,amount,drCr,description,reference[,bankTxnId]
var requiredColumns = []string{"date", "valuedate", "amount", "drcr", "description", "reference"}

// AdminImportBankStatementHandler handles
// POST /admin/reconciliation/bank-accounts/:id/import — requires
// RECONCILIATION_MANAGE. Accepts a CSV file upload (multipart form field
// "file"). Each row becomes a BankTransaction, and autoMatch is run
// immediately per-row so a straightforward exact-reference match doesn't
// need a separate manual step. The architecture (a stored BankAccount +
// a discrete BankStatementImport batch + individual BankTransaction rows)
// is deliberately format-agnostic — a future direct bank-API feed would
// populate the same three tables without a schema change, only a new
// import path alongside this CSV one.
func AdminImportBankStatementHandler(c *gin.Context) {
	bankAccountID, _ := strconv.Atoi(c.Param("id"))
	var account models.BankAccount
	if err := database.DB.First(&account, bankAccountID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Bank account not found"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "A CSV file upload (\"file\" field) is required"})
		return
	}
	if fileHeader.Size > 5*1024*1024 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "File too large (max 5MB)"})
		return
	}
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Could not read the uploaded file"})
		return
	}
	defer file.Close()

	rows, err := parseStatementCSV(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	if len(rows) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "No data rows found in the file"})
		return
	}

	adminID, _ := c.Get("admin_id")
	adminUsername := c.GetString("admin_username")
	id, _ := adminID.(uint)

	imp := models.BankStatementImport{
		BankAccountID:     account.ID,
		FileName:          fileHeader.Filename,
		RowCount:          len(rows),
		ImportedBy:        adminUsername,
		ImportedByAdminID: id,
	}
	if err := database.DB.Create(&imp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Failed to record import batch"})
		return
	}

	matched, exceptions := 0, 0
	for _, row := range rows {
		txn := models.BankTransaction{
			ImportID:      imp.ID,
			BankAccountID: account.ID,
			BankTxnID:     row.bankTxnID,
			TxnDate:       row.txnDate,
			ValueDate:     row.valueDate,
			AmountPaise:   row.amountPaise,
			DrCr:          row.drCr,
			Description:   row.description,
			Reference:     row.reference,
			Status:        models.BankTxnStatusUnmatched,
		}
		if err := database.DB.Create(&txn).Error; err != nil {
			continue
		}
		autoMatch(&txn)
		switch txn.Status {
		case models.BankTxnStatusMatched, models.BankTxnStatusReconciled:
			matched++
		case models.BankTxnStatusException:
			exceptions++
		}
	}

	audit.RecordDetailed(c, audit.Entry{
		ActorType: "admin", ActorID: id, ActorRef: adminUsername, ActorRole: c.GetString("admin_role"),
		Action: "bank_statement.imported", TargetType: "bank_statement_import", TargetID: strconv.Itoa(int(imp.ID)),
		Result: audit.ResultSuccess,
		Details: fileHeader.Filename + ": " + strconv.Itoa(len(rows)) + " rows, " + strconv.Itoa(matched) +
			" auto-matched, " + strconv.Itoa(exceptions) + " exceptions",
	})

	c.JSON(http.StatusCreated, gin.H{
		"importId":    imp.ID,
		"rowCount":    len(rows),
		"autoMatched": matched,
		"exceptions":  exceptions,
	})
}

type statementRow struct {
	txnDate     time.Time
	valueDate   time.Time
	amountPaise int64
	drCr        string
	description string
	reference   string
	bankTxnID   string
}

func parseStatementCSV(r io.Reader) ([]statementRow, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, errors.New("could not read CSV header row")
	}
	colIndex := map[string]int{}
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, required := range requiredColumns {
		if _, ok := colIndex[required]; !ok {
			return nil, errors.New("CSV is missing required column: " + required +
				" (expected: date, valueDate, amount, drCr, description, reference)")
		}
	}
	bankTxnIdx, hasBankTxnID := colIndex["banktxnid"]

	var rows []statementRow
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("could not parse CSV: " + err.Error())
		}
		if len(record) < len(requiredColumns) {
			continue
		}

		txnDate, err1 := parseFlexibleDate(record[colIndex["date"]])
		valueDate, err2 := parseFlexibleDate(record[colIndex["valuedate"]])
		amount, err3 := strconv.ParseFloat(strings.TrimSpace(record[colIndex["amount"]]), 64)
		if err1 != nil || err2 != nil || err3 != nil {
			continue // skip malformed rows rather than aborting the whole import
		}
		drCr := strings.ToUpper(strings.TrimSpace(record[colIndex["drcr"]]))
		if drCr != "DR" && drCr != "CR" {
			continue
		}

		row := statementRow{
			txnDate:     txnDate,
			valueDate:   valueDate,
			amountPaise: int64(math.Round(amount * 100)),
			drCr:        drCr,
			description: strings.TrimSpace(record[colIndex["description"]]),
			reference:   strings.TrimSpace(record[colIndex["reference"]]),
		}
		if hasBankTxnID && bankTxnIdx < len(record) {
			row.bankTxnID = strings.TrimSpace(record[bankTxnIdx])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func parseFlexibleDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", "02-01-2006", "02/01/2006", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("unrecognized date: " + s)
}
