import 'dart:typed_data';
import 'package:intl/intl.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import '../../models/active_loan.dart';
import '../../models/transaction.dart';

/// Shared PDF builders for the customer app's payment receipt and the
/// statement/export actions on My Loans, Transaction History and EMI
/// Schedule. Mirrors the header/branding conventions used by
/// `LoanAgreementPdf` and the documents screen's inline PDF builder so
/// every generated PDF in the app looks consistent.
class StatementPdfGenerator {
  StatementPdfGenerator._();

  static final _currency = NumberFormat.currency(locale: 'en_IN', symbol: 'Rs. ', decimalDigits: 0);
  static final _dateFmt = DateFormat('d MMM yyyy, h:mm a');

  static pw.Widget _header(String title) => pw.Column(
        crossAxisAlignment: pw.CrossAxisAlignment.start,
        children: [
          pw.Row(
            mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
            children: [
              pw.Text('NBFC PREMIUM', style: pw.TextStyle(fontSize: 16, fontWeight: pw.FontWeight.bold, color: PdfColor.fromInt(0xFF0057FF))),
              pw.Text(_dateFmt.format(DateTime.now()), style: const pw.TextStyle(fontSize: 9, color: PdfColors.grey700)),
            ],
          ),
          pw.SizedBox(height: 4),
          pw.Text(title, style: pw.TextStyle(fontSize: 18, fontWeight: pw.FontWeight.bold)),
          pw.Divider(height: 20),
        ],
      );

  static pw.Widget _footer() => pw.Padding(
        padding: const pw.EdgeInsets.only(top: 16),
        child: pw.Text(
          'This is a system-generated document from NBFC Premium and does not require a physical signature.',
          style: const pw.TextStyle(fontSize: 8, color: PdfColors.grey600),
        ),
      );

  static Future<Uint8List> buildPaymentReceipt({
    required String referenceNumber,
    required String loanProductName,
    required String loanId,
    required double amount,
    required DateTime date,
    required String paymentMethod,
    required String status,
  }) async {
    final pdf = pw.Document();
    pdf.addPage(
      pw.Page(
        pageFormat: PdfPageFormat.a5,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => pw.Column(
          crossAxisAlignment: pw.CrossAxisAlignment.start,
          children: [
            _header('Payment Receipt'),
            _kv('Loan', loanProductName),
            _kv('Loan ID', loanId),
            _kv('Amount Paid', _currency.format(amount)),
            _kv('Reference No.', referenceNumber),
            _kv('Date & Time', _dateFmt.format(date)),
            _kv('Payment Method', paymentMethod),
            _kv('Status', status),
            _footer(),
          ],
        ),
      ),
    );
    return pdf.save();
  }

  static Future<Uint8List> buildLoansStatement(List<ActiveLoan> loans) async {
    final pdf = pw.Document();
    pdf.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          _header('My Loans Statement'),
          pw.TableHelper.fromTextArray(
            headers: ['Loan', 'Loan ID', 'Principal', 'Outstanding', 'Next EMI', 'Rate'],
            data: loans
                .map((l) => [
                      l.productName,
                      l.loanId,
                      _currency.format(l.principal),
                      _currency.format(l.outstanding),
                      _currency.format(l.nextEmiAmount),
                      '${l.interestRate}%',
                    ])
                .toList(),
            headerStyle: pw.TextStyle(fontWeight: pw.FontWeight.bold, fontSize: 9),
            cellStyle: const pw.TextStyle(fontSize: 9),
            cellAlignment: pw.Alignment.centerLeft,
            headerDecoration: const pw.BoxDecoration(color: PdfColors.grey200),
          ),
          _footer(),
        ],
      ),
    );
    return pdf.save();
  }

  static Future<Uint8List> buildTransactionsStatement(List<AppTransaction> transactions) async {
    final pdf = pw.Document();
    final dateOnly = DateFormat('d MMM yyyy');
    pdf.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          _header('Transaction History'),
          pw.TableHelper.fromTextArray(
            headers: ['Date', 'Description', 'Loan ID', 'Amount', 'Status'],
            data: transactions
                .map((t) => [
                      dateOnly.format(t.date),
                      t.description,
                      t.loanId,
                      _currency.format(t.amount),
                      t.status.name,
                    ])
                .toList(),
            headerStyle: pw.TextStyle(fontWeight: pw.FontWeight.bold, fontSize: 9),
            cellStyle: const pw.TextStyle(fontSize: 9),
            cellAlignment: pw.Alignment.centerLeft,
            headerDecoration: const pw.BoxDecoration(color: PdfColors.grey200),
          ),
          _footer(),
        ],
      ),
    );
    return pdf.save();
  }

  static Future<Uint8List> buildEmiSchedule({
    required ActiveLoan loan,
    required List<(int month, DateTime dueDate, double emi, double principal, double interest, double balance, bool paid)> rows,
  }) async {
    final pdf = pw.Document();
    final dateOnly = DateFormat('MMM yyyy');
    pdf.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          _header('EMI Schedule — ${loan.productName}'),
          _kv('Loan ID', loan.loanId),
          _kv('Tenure', '${loan.tenureMonths} months'),
          _kv('Interest Rate', '${loan.interestRate}% p.a.'),
          pw.SizedBox(height: 12),
          pw.TableHelper.fromTextArray(
            headers: ['#', 'Due', 'EMI', 'Principal', 'Interest', 'Balance', 'Status'],
            data: rows
                .map((r) => [
                      '${r.$1}',
                      dateOnly.format(r.$2),
                      _currency.format(r.$3),
                      _currency.format(r.$4),
                      _currency.format(r.$5),
                      _currency.format(r.$6),
                      r.$7 ? 'Paid' : 'Upcoming',
                    ])
                .toList(),
            headerStyle: pw.TextStyle(fontWeight: pw.FontWeight.bold, fontSize: 8),
            cellStyle: const pw.TextStyle(fontSize: 8),
            cellAlignment: pw.Alignment.centerLeft,
            headerDecoration: const pw.BoxDecoration(color: PdfColors.grey200),
          ),
          _footer(),
        ],
      ),
    );
    return pdf.save();
  }

  /// Real backend-data variant of [buildEmiSchedule] — for the schedule
  /// returned by `GET /auth/loans/:id/emi-schedule`
  /// (`internal/loans/emi_handler.go`), which has no running "balance"
  /// concept and installment `status` of `pending`/`paid`/`overdue`
  /// rather than a simple paid flag.
  static Future<Uint8List> buildEmiScheduleFromInstallments({
    required int loanId,
    required int tenureMonths,
    required double interestRatePercent,
    required List<(int installmentNumber, DateTime dueDate, double amount, double principal, double interest, String status)> installments,
  }) async {
    final pdf = pw.Document();
    final dateOnly = DateFormat('d MMM yyyy');
    pdf.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          _header('EMI Schedule — NBFC-APP-$loanId'),
          _kv('Loan ID', 'NBFC-APP-$loanId'),
          _kv('Tenure', '$tenureMonths months'),
          _kv('Interest Rate', '$interestRatePercent% p.a.'),
          pw.SizedBox(height: 12),
          pw.TableHelper.fromTextArray(
            headers: ['#', 'Due', 'EMI', 'Principal', 'Interest', 'Status'],
            data: installments
                .map((r) => [
                      '${r.$1}',
                      dateOnly.format(r.$2),
                      _currency.format(r.$3),
                      _currency.format(r.$4),
                      _currency.format(r.$5),
                      r.$6 == 'paid' ? 'Paid' : (r.$6 == 'overdue' ? 'Overdue' : 'Upcoming'),
                    ])
                .toList(),
            headerStyle: pw.TextStyle(fontWeight: pw.FontWeight.bold, fontSize: 8),
            cellStyle: const pw.TextStyle(fontSize: 8),
            cellAlignment: pw.Alignment.centerLeft,
            headerDecoration: const pw.BoxDecoration(color: PdfColors.grey200),
          ),
          _footer(),
        ],
      ),
    );
    return pdf.save();
  }

  static pw.Widget _kv(String label, String value) => pw.Padding(
        padding: const pw.EdgeInsets.only(bottom: 4),
        child: pw.Row(
          children: [
            pw.SizedBox(width: 120, child: pw.Text(label, style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700))),
            pw.Expanded(child: pw.Text(value, style: pw.TextStyle(fontSize: 10, fontWeight: pw.FontWeight.bold))),
          ],
        ),
      );
}
