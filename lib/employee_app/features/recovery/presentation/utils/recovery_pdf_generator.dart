import 'dart:io';
import 'dart:typed_data';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import '../../../../core/utils/receipt_pdf_generator.dart';
import '../../domain/entities/recovery_case_entity.dart';

/// Builds recovery-flow PDFs (field-visit reports, recovery reports and
/// payment receipts) mirroring the layout/branding conventions established
/// by `PdfGenerator` in the Verification module.
class RecoveryPdfGenerator {
  RecoveryPdfGenerator._();

  static Future<File> generateVisitReport(RecoveryCaseEntity caseItem, VisitRecord record) async {
    final doc = pw.Document();
    final dateFmt = DateFormat('EEE, d MMM yyyy · h:mm a');
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: 'Rs. ', decimalDigits: 0);

    pw.MemoryImage? signature;
    if (record.signaturePath != null) {
      final file = File(record.signaturePath!);
      if (await file.exists()) signature = pw.MemoryImage(await file.readAsBytes());
    }
    final photoImages = <pw.MemoryImage>[];
    for (final path in record.photoPaths) {
      final file = File(path);
      if (await file.exists()) photoImages.add(pw.MemoryImage(await file.readAsBytes()));
    }

    doc.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          pw.Row(
            mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
            children: [
              pw.Text('ONEFIN — Recovery Visit Report', style: pw.TextStyle(fontSize: 18, fontWeight: pw.FontWeight.bold)),
              pw.Text(dateFmt.format(record.checkInTime), style: const pw.TextStyle(fontSize: 10)),
            ],
          ),
          pw.SizedBox(height: 4),
          pw.Text('Case ${caseItem.id}  ·  Loan ${caseItem.loanId}', style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700)),
          pw.Divider(height: 24),
          _sectionTitle('Customer Details'),
          _kv('Customer Name', caseItem.customerName),
          _kv('Phone', caseItem.customerPhone),
          _kv('Address', caseItem.address),
          _kv('Loan Type', caseItem.loanType.label),
          pw.SizedBox(height: 16),
          _sectionTitle('Loan Summary'),
          _kv('Outstanding', currency.format(caseItem.outstandingAmount)),
          _kv('DPD', '${caseItem.dpd} days'),
          _kv('Overdue EMIs', '${caseItem.overdueEmiCount}'),
          pw.SizedBox(height: 16),
          _sectionTitle('Visit Details'),
          _kv('Check-In Time', dateFmt.format(record.checkInTime)),
          _kv('Location (Lat, Lng)', '${record.latitude.toStringAsFixed(6)}, ${record.longitude.toStringAsFixed(6)}'),
          _kv('Customer Present', record.customerPresent ? 'Yes' : 'No'),
          _kv('Notes', record.notes),
          if (photoImages.isNotEmpty) ...[
            pw.SizedBox(height: 16),
            _sectionTitle('Visit Photos'),
            pw.Wrap(
              spacing: 8,
              runSpacing: 8,
              children: photoImages
                  .map((img) => pw.Container(height: 100, width: 100, child: pw.Image(img, fit: pw.BoxFit.cover)))
                  .toList(),
            ),
          ],
          pw.SizedBox(height: 24),
          pw.Column(
            crossAxisAlignment: pw.CrossAxisAlignment.start,
            children: [
              if (signature != null) pw.Container(height: 60, width: 160, child: pw.Image(signature)),
              pw.Container(
                width: 160,
                decoration: const pw.BoxDecoration(border: pw.Border(top: pw.BorderSide())),
                padding: const pw.EdgeInsets.only(top: 4),
                child: pw.Text('Customer / Field Officer Signature', style: const pw.TextStyle(fontSize: 9)),
              ),
            ],
          ),
        ],
      ),
    );

    return _save(doc, 'recovery_visit_${caseItem.id}_${record.checkInTime.millisecondsSinceEpoch}');
  }

  static Future<File> generateReportPdf(RecoveryCaseEntity caseItem, RecoveryReport report) async {
    final doc = pw.Document();
    final dateFmt = DateFormat('EEE, d MMM yyyy · h:mm a');
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: 'Rs. ', decimalDigits: 0);

    final photoImages = <pw.MemoryImage>[];
    for (final path in report.photoPaths) {
      final file = File(path);
      if (await file.exists()) photoImages.add(pw.MemoryImage(await file.readAsBytes()));
    }

    doc.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          pw.Row(
            mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
            children: [
              pw.Text('ONEFIN — Recovery Report', style: pw.TextStyle(fontSize: 18, fontWeight: pw.FontWeight.bold)),
              pw.Text(dateFmt.format(report.submittedAt), style: const pw.TextStyle(fontSize: 10)),
            ],
          ),
          pw.SizedBox(height: 4),
          pw.Text('Case ${caseItem.id}  ·  Loan ${caseItem.loanId}', style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700)),
          pw.Divider(height: 24),
          _sectionTitle('Customer Details'),
          _kv('Customer Name', caseItem.customerName),
          _kv('Phone', caseItem.customerPhone),
          _kv('Outstanding', currency.format(caseItem.outstandingAmount)),
          pw.SizedBox(height: 16),
          _sectionTitle('Report Summary'),
          _kv('Status', report.status.label),
          _kv('Risk Assessment', report.riskAssessment.label),
          _kv('Priority', report.priority.label),
          _kv('Remarks', report.remarks),
          _kv('Recommendation', report.recommendation),
          _kv('Next Action Date', '${report.nextAction.day}/${report.nextAction.month}/${report.nextAction.year}'),
          if (photoImages.isNotEmpty) ...[
            pw.SizedBox(height: 16),
            _sectionTitle('Supporting Photos'),
            pw.Wrap(
              spacing: 8,
              runSpacing: 8,
              children: photoImages
                  .map((img) => pw.Container(height: 100, width: 100, child: pw.Image(img, fit: pw.BoxFit.cover)))
                  .toList(),
            ),
          ],
        ],
      ),
    );

    return _save(doc, 'recovery_report_${caseItem.id}_${report.submittedAt.millisecondsSinceEpoch}');
  }

  /// Payment receipt for a recovery case — delegates to the shared
  /// [ReceiptPdfGenerator] but adds recovery-specific DPD context.
  static Future<File> generatePaymentReceiptPdf({
    required RecoveryCaseEntity caseItem,
    required String receiptNumber,
    required double amount,
    required String paymentMode,
    required DateTime date,
  }) {
    return ReceiptPdfGenerator.generatePaymentReceipt(
      receiptNumber: receiptNumber,
      customerName: caseItem.customerName,
      loanAccountNumber: caseItem.loanId,
      amount: amount,
      paymentMode: paymentMode,
      date: date,
      officerName: 'Recovery Officer',
      extraLabel: 'DPD',
      extraValue: '${caseItem.dpd} days',
    );
  }

  static Future<File> _save(pw.Document doc, String baseName) async {
    final Uint8List bytes = await doc.save();
    final dir = await getApplicationDocumentsDirectory();
    final file = File('${dir.path}/$baseName.pdf');
    await file.writeAsBytes(bytes);
    return file;
  }

  static pw.Widget _sectionTitle(String title) => pw.Padding(
        padding: const pw.EdgeInsets.only(bottom: 8),
        child: pw.Text(title, style: pw.TextStyle(fontSize: 13, fontWeight: pw.FontWeight.bold)),
      );

  static pw.Widget _kv(String label, String value) => pw.Padding(
        padding: const pw.EdgeInsets.only(bottom: 4),
        child: pw.Row(
          children: [
            pw.SizedBox(width: 130, child: pw.Text(label, style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700))),
            pw.Expanded(child: pw.Text(value.isEmpty ? '—' : value, style: const pw.TextStyle(fontSize: 10))),
          ],
        ),
      );
}
