import 'dart:io';
import 'dart:typed_data';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;

/// Shared mock payment-receipt PDF builder used by both the Recovery and
/// Collections flows so the two receipt shapes stay visually consistent.
/// Mirrors the header/branding conventions of [PdfGenerator].
class ReceiptPdfGenerator {
  ReceiptPdfGenerator._();

  static Future<File> generatePaymentReceipt({
    required String receiptNumber,
    required String customerName,
    required String loanAccountNumber,
    required double amount,
    required String paymentMode,
    required DateTime date,
    String? officerName,
    String? extraLabel,
    String? extraValue,
  }) async {
    final doc = pw.Document();
    final dateFmt = DateFormat('EEE, d MMM yyyy · h:mm a');
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: 'Rs. ', decimalDigits: 0);

    doc.addPage(
      pw.Page(
        pageFormat: PdfPageFormat.a5,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => pw.Column(
          crossAxisAlignment: pw.CrossAxisAlignment.start,
          children: [
            pw.Row(
              mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
              children: [
                pw.Text('ONEFIN — Payment Receipt', style: pw.TextStyle(fontSize: 16, fontWeight: pw.FontWeight.bold)),
                pw.Text(dateFmt.format(date), style: const pw.TextStyle(fontSize: 9)),
              ],
            ),
            pw.Divider(height: 20),
            _kv('Receipt Number', receiptNumber),
            _kv('Customer Name', customerName),
            _kv('Loan A/C Number', loanAccountNumber),
            _kv('Amount', currency.format(amount)),
            _kv('Payment Mode', paymentMode),
            if (extraLabel != null && extraValue != null) _kv(extraLabel, extraValue),
            if (officerName != null) _kv('Collected By', officerName),
            pw.SizedBox(height: 24),
            pw.Divider(),
            pw.SizedBox(height: 8),
            pw.Text(
              'This is a system-generated receipt for the payment recorded against the above loan account.',
              style: const pw.TextStyle(fontSize: 8, color: PdfColors.grey600),
            ),
          ],
        ),
      ),
    );

    final Uint8List bytes = await doc.save();
    final dir = await getApplicationDocumentsDirectory();
    final file = File('${dir.path}/receipt_${receiptNumber.replaceAll(RegExp(r'[^A-Za-z0-9]'), '_')}.pdf');
    await file.writeAsBytes(bytes);
    return file;
  }

  static pw.Widget _kv(String label, String value) => pw.Padding(
        padding: const pw.EdgeInsets.only(bottom: 6),
        child: pw.Row(
          children: [
            pw.SizedBox(width: 120, child: pw.Text(label, style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700))),
            pw.Expanded(child: pw.Text(value, style: pw.TextStyle(fontSize: 10, fontWeight: pw.FontWeight.bold))),
          ],
        ),
      );
}
