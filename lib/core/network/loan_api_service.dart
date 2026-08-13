import 'api_client.dart';
import '../../models/loan_application.dart';
import '../../models/uploaded_document.dart';

/// Wraps the backend's protected `/auth/loans/*` endpoints
/// (`internal/loans/customer_handler.go`) — submitting a loan application
/// and reading the logged-in customer's own applications back.
class LoanApiService {
  LoanApiService._();

  /// `category` must be one of the backend's loan category strings (in
  /// practice, the same `LoanProduct.name` shown in the UI — see
  /// `internal/loans/service.go`'s `autoApproveCategories` map, which
  /// keys on "Instant Loan" / "Personal Loan" exactly).
  static Future<LoanApplication> apply(
    String token, {
    required String category,
    required double amountRequested,
    required int tenureMonths,
    required double monthlyIncome,
    required String purpose,
    required String applicantName,
    required String applicantPhone,
    required String addressLine,
    required String city,
    String? timingPreference,
    DateTime? visitDate,
    String? visitTimeSlot,
  }) async {
    // A non-2xx response here throws ApiException with the backend's
    // eligibility-engine message (internal/loans/eligibility.go) already
    // in .message — e.g. "This EMI would be more than 50% of your
    // declared monthly income..." — callers show that directly rather
    // than a generic failure.
    final res = await ApiClient.post('/auth/loans/apply', {
      'category': category,
      'amountRequested': amountRequested,
      'tenureMonths': tenureMonths,
      'monthlyIncome': monthlyIncome,
      'purpose': purpose,
      'applicantName': applicantName,
      'applicantPhone': applicantPhone,
      'addressLine': addressLine,
      'city': city,
      if (timingPreference != null) 'timingPreference': timingPreference,
      if (visitDate != null) 'visitDate': '${visitDate.year.toString().padLeft(4, '0')}-${visitDate.month.toString().padLeft(2, '0')}-${visitDate.day.toString().padLeft(2, '0')}',
      if (visitTimeSlot != null) 'visitTimeSlot': visitTimeSlot,
    }, token: token);
    return LoanApplication.fromJson(res);
  }

  static Future<List<LoanApplication>> mine(String token) async {
    final res = await ApiClient.getList('/auth/loans/mine', token: token);
    return res.map((e) => LoanApplication.fromJson(e as Map<String, dynamic>)).toList();
  }

  static Future<LoanApplication> detail(String token, int id) async {
    final res = await ApiClient.get('/auth/loans/$id', token: token);
    return LoanApplication.fromJson(res);
  }

  /// Requests a top-up on an existing disbursed loan
  /// (`POST /auth/loans/:id/topup`, `internal/loans/customer_handler.go`
  /// `TopUpLoanHandler`). Creates a brand new [LoanApplication] linked to
  /// [loanId] via `parentLoanId`, run through the same verification/sanction
  /// pipeline as any other application.
  static Future<LoanApplication> topUp(
    String token,
    int loanId, {
    required double amount,
    required int tenureMonths,
    required String purpose,
  }) async {
    final res = await ApiClient.post('/auth/loans/$loanId/topup', {
      'amount': amount,
      'tenureMonths': tenureMonths,
      'purpose': purpose,
    }, token: token);
    return LoanApplication.fromJson(res);
  }

  /// eSigns the given system-generated letter for this loan
  /// (`POST /auth/loans/:id/esign`, `internal/loans/esign_handler.go`
  /// `CustomerEsignLetterHandler`). `letterType` must be exactly
  /// `"Sanction Letter"` or `"Disbursement Letter"`. `signaturePngBase64`
  /// is the raw PNG bytes of the signature-pad capture, base64-encoded,
  /// with NO `data:` prefix. The backend regenerates that letter's PDF
  /// with the signature embedded directly into it and returns the newly
  /// created `Document` in full — including `dataBase64` — so the caller
  /// has the signed PDF immediately, with no need to re-fetch the
  /// document list. Re-signing (calling this again for the same letter)
  /// replaces the previous signed copy.
  static Future<UploadedDocument> esign(
    String token,
    int loanId, {
    required String letterType,
    required String signaturePngBase64,
  }) async {
    final res = await ApiClient.post('/auth/loans/$loanId/esign', {
      'letterType': letterType,
      'signatureBase64': signaturePngBase64,
    }, token: token);
    return UploadedDocument.fromJson(res);
  }
}
