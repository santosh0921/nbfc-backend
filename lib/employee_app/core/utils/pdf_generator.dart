import 'dart:io';
import 'dart:typed_data';
import 'package:intl/intl.dart';
import 'package:path_provider/path_provider.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;
import '../../features/visit_verification/domain/entities/visit_record.dart';

class PdfGenerator {
  PdfGenerator._();

  static Future<File> generateVisitReport(VisitRecord record) async {
    final doc = pw.Document();
    final dateFmt = DateFormat('EEE, d MMM yyyy · h:mm a');

    pw.MemoryImage? geoImage;
    if (record.geoPhoto != null) {
      final file = File(record.geoPhoto!.filePath);
      if (await file.exists()) geoImage = pw.MemoryImage(await file.readAsBytes());
    }
    pw.MemoryImage? customerSignature;
    if (record.customerSignaturePath != null) {
      final file = File(record.customerSignaturePath!);
      if (await file.exists()) customerSignature = pw.MemoryImage(await file.readAsBytes());
    }
    pw.MemoryImage? employeeSignature;
    if (record.employeeSignaturePath != null) {
      final file = File(record.employeeSignaturePath!);
      if (await file.exists()) employeeSignature = pw.MemoryImage(await file.readAsBytes());
    }

    doc.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        margin: const pw.EdgeInsets.all(28),
        build: (context) => [
          pw.Row(
            mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
            children: [
              pw.Text('ONEFIN — Verification Report', style: pw.TextStyle(fontSize: 18, fontWeight: pw.FontWeight.bold)),
              pw.Text(dateFmt.format(record.submittedAt), style: const pw.TextStyle(fontSize: 10)),
            ],
          ),
          pw.SizedBox(height: 4),
          pw.Text('Case ${record.caseId}', style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700)),
          pw.Divider(height: 24),
          _sectionTitle('Customer Details'),
          _kv('Customer Name', record.customerName),
          _kv('Loan Type', record.loanType),
          _kv('Occupation', record.occupation),
          _kv('Monthly Income', record.monthlyIncome),
          _kv('Remarks', record.remarks),
          pw.SizedBox(height: 16),
          _sectionTitle('Witness Details'),
          if (record.witnesses.isEmpty) pw.Text('No witnesses added', style: const pw.TextStyle(fontSize: 10)),
          ...record.witnesses.map(
            (w) => pw.Padding(
              padding: const pw.EdgeInsets.only(bottom: 6),
              child: pw.Text(
                '${w.name}  ·  ${w.relation}  ·  ${w.phone}  ·  DOB ${w.dob}'
                '${w.documentPath != null ? '  ·  ID document captured' : ''}',
                style: const pw.TextStyle(fontSize: 10),
              ),
            ),
          ),
          pw.SizedBox(height: 16),
          _sectionTitle('Documents Verified'),
          if (record.documents.isEmpty) pw.Text('No documents captured', style: const pw.TextStyle(fontSize: 10)),
          ...record.documents.map(
            (d) => pw.Bullet(text: d.type, style: const pw.TextStyle(fontSize: 10)),
          ),
          pw.SizedBox(height: 16),
          if (geoImage != null) ...[
            _sectionTitle('Geotagged Site Photo'),
            pw.Container(height: 200, child: pw.Image(geoImage, fit: pw.BoxFit.cover)),
            pw.SizedBox(height: 4),
            pw.Text(
              'Lat ${record.geoPhoto!.latitude.toStringAsFixed(6)}, Lng ${record.geoPhoto!.longitude.toStringAsFixed(6)}'
              '  ·  ${dateFmt.format(record.geoPhoto!.capturedAt)}',
              style: const pw.TextStyle(fontSize: 9, color: PdfColors.grey700),
            ),
            pw.SizedBox(height: 16),
          ],
          _sectionTitle('Recommendation'),
          pw.Text(record.recommendation, style: const pw.TextStyle(fontSize: 11)),
          pw.SizedBox(height: 24),
          pw.Row(
            mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
            children: [
              pw.Column(
                crossAxisAlignment: pw.CrossAxisAlignment.start,
                children: [
                  if (customerSignature != null) pw.Container(height: 60, width: 140, child: pw.Image(customerSignature)),
                  pw.Container(width: 140, decoration: const pw.BoxDecoration(border: pw.Border(top: pw.BorderSide())), padding: const pw.EdgeInsets.only(top: 4), child: pw.Text('Customer Signature', style: const pw.TextStyle(fontSize: 9))),
                ],
              ),
              pw.Column(
                crossAxisAlignment: pw.CrossAxisAlignment.start,
                children: [
                  if (employeeSignature != null) pw.Container(height: 60, width: 140, child: pw.Image(employeeSignature)),
                  pw.Container(width: 140, decoration: const pw.BoxDecoration(border: pw.Border(top: pw.BorderSide())), padding: const pw.EdgeInsets.only(top: 4), child: pw.Text('Field Officer Signature', style: const pw.TextStyle(fontSize: 9))),
                ],
              ),
            ],
          ),
        ],
      ),
    );

    final Uint8List bytes = await doc.save();
    final dir = await getApplicationDocumentsDirectory();
    final file = File('${dir.path}/visit_report_${record.id}.pdf');
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
            pw.SizedBox(width: 120, child: pw.Text(label, style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700))),
            pw.Expanded(child: pw.Text(value.isEmpty ? '—' : value, style: const pw.TextStyle(fontSize: 10))),
          ],
        ),
      );
}
