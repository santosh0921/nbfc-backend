import 'dart:typed_data';
import 'package:intl/intl.dart';
import 'package:pdf/pdf.dart';
import 'package:pdf/widgets.dart' as pw;

/// Builds the signed loan agreement PDF: Key Facts Statement, the full
/// terms & conditions text the user scrolled through, and their captured
/// e-signature — a real signed document, not just a KFS summary.
class LoanAgreementPdf {
  LoanAgreementPdf._();

  static Future<Uint8List> build({
    required String applicantName,
    required String productName,
    required double loanAmount,
    required double tenureMonths,
    required double interestRate,
    required double aprRate,
    required String processingFee,
    required double emi,
    required String applicationId,
    required Uint8List signatureImage,
  }) async {
    final pdf = pw.Document();
    final currency = NumberFormat.currency(locale: 'en_IN', symbol: 'Rs. ', decimalDigits: 0);
    final signature = pw.MemoryImage(signatureImage);
    final signedOn = DateFormat('d MMM yyyy, h:mm a').format(DateTime.now());

    pw.Widget kfsRow(String label, String value) => pw.Padding(
          padding: const pw.EdgeInsets.symmetric(vertical: 4),
          child: pw.Row(
            mainAxisAlignment: pw.MainAxisAlignment.spaceBetween,
            children: [
              pw.Text(label, style: const pw.TextStyle(fontSize: 11)),
              pw.Text(value, style: pw.TextStyle(fontSize: 11, fontWeight: pw.FontWeight.bold)),
            ],
          ),
        );

    pdf.addPage(
      pw.MultiPage(
        pageFormat: PdfPageFormat.a4,
        build: (context) => [
          pw.Text('Loan Agreement & Key Facts Statement', style: pw.TextStyle(fontSize: 20, fontWeight: pw.FontWeight.bold)),
          pw.SizedBox(height: 4),
          pw.Text('Application ID: $applicationId', style: const pw.TextStyle(fontSize: 10, color: PdfColors.grey700)),
          pw.SizedBox(height: 16),
          pw.Container(
            padding: const pw.EdgeInsets.all(12),
            decoration: pw.BoxDecoration(border: pw.Border.all(color: PdfColors.grey400), borderRadius: pw.BorderRadius.circular(6)),
            child: pw.Column(children: [
              kfsRow('Applicant Name', applicantName),
              pw.Divider(),
              kfsRow('Loan Product', productName),
              pw.Divider(),
              kfsRow('Loan Amount', currency.format(loanAmount)),
              pw.Divider(),
              kfsRow('Tenure', '${tenureMonths.round()} months'),
              pw.Divider(),
              kfsRow('Interest Rate', '$interestRate% p.a.'),
              pw.Divider(),
              kfsRow('Annual Percentage Rate (APR)', '$aprRate% p.a.'),
              pw.Divider(),
              kfsRow('Processing Fee', processingFee),
              pw.Divider(),
              kfsRow('Overdue Charges', '2% p.m. on overdue amount'),
              pw.Divider(),
              kfsRow('Estimated Monthly EMI', currency.format(emi)),
            ]),
          ),
          pw.SizedBox(height: 20),
          pw.Text('Terms & Conditions', style: pw.TextStyle(fontSize: 15, fontWeight: pw.FontWeight.bold)),
          pw.SizedBox(height: 8),
          pw.Text(_termsText, style: const pw.TextStyle(fontSize: 9.5, lineSpacing: 2)),
          pw.SizedBox(height: 28),
          pw.Text('Borrower Acknowledgement', style: pw.TextStyle(fontSize: 13, fontWeight: pw.FontWeight.bold)),
          pw.SizedBox(height: 8),
          pw.Text(
            'I, $applicantName, confirm that I have read and understood the Key Facts Statement and the '
            'Terms & Conditions above, and I agree to be bound by them for this loan application.',
            style: const pw.TextStyle(fontSize: 10),
          ),
          pw.SizedBox(height: 16),
          pw.Container(
            height: 90,
            width: 220,
            decoration: pw.BoxDecoration(border: pw.Border.all(color: PdfColors.grey400)),
            child: pw.Image(signature, fit: pw.BoxFit.contain),
          ),
          pw.SizedBox(height: 6),
          pw.Text('Signed electronically on $signedOn', style: const pw.TextStyle(fontSize: 9, color: PdfColors.grey700)),
        ],
      ),
    );

    return pdf.save();
  }

  static const _termsText = '''
1. LOAN DISBURSAL: The sanctioned loan amount will be disbursed to the borrower's registered bank account after successful completion of KYC verification, video KYC (and property verification where applicable), and internal credit approval.

2. INTEREST & CHARGES: Interest will be charged on a reducing balance basis at the rate stated in the Key Facts Statement above. The Annual Percentage Rate (APR) includes interest and all applicable processing charges as mandated by RBI's Key Facts Statement guidelines (2024).

3. REPAYMENT: The borrower agrees to repay the loan through Equated Monthly Instalments (EMIs) as per the agreed schedule, via the registered bank account (NACH/auto-debit) or any other payment method made available in the app.

4. OVERDUE CHARGES: In the event of delayed or missed EMI payments, overdue charges as stated in the Key Facts Statement will apply on the overdue amount, calculated from the due date until the date of actual payment.

5. PREPAYMENT & FORECLOSURE: The borrower may prepay or foreclose the loan, in part or in full, subject to the applicable prepayment charges (if any) disclosed at the time of such request, in accordance with RBI's Fair Practices Code.

6. CREDIT BUREAU REPORTING: The borrower authorizes the lender to obtain credit information from CIBIL and other credit information companies for assessment of this application, and to report the loan account status (including any delinquency) to such bureaus on an ongoing basis.

7. COMMUNICATION CONSENT: The borrower consents to receive application status updates, payment reminders, and other service communication via SMS, email, WhatsApp, and push notification.

8. GRIEVANCE REDRESSAL: Any grievance regarding this loan may be raised through the app's Support section or with the Nodal Officer, whose details are available under Compliance in the app. The lender will resolve grievances within the timelines prescribed by the RBI Fair Practices Code.

9. GOVERNING LAW: This agreement is governed by the laws of India and is subject to the guidelines issued by the Reserve Bank of India for Non-Banking Financial Companies (NBFCs) and Digital Lending, as amended from time to time.

10. DATA PRIVACY: Personal and financial information collected during this application will be used solely for loan processing, underwriting, servicing, and regulatory compliance, and will not be shared with third parties except as required by law or for the purposes disclosed in the app's Privacy Policy.

By signing below, the borrower acknowledges having read, understood, and agreed to all the terms and conditions stated above in their entirety.
''';
}
