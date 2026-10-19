import 'api_client.dart';
import '../../models/branch.dart';
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
    // Home Loan only — real-world builder/property details a
    // construction-linked purchase needs (internal/loans/customer_handler.go).
    String? builderName,
    String? projectName,
    String? propertyAddress,
    double? agreementValue,
    String? constructionStage,
    // Gold Loan only — the customer's own estimate; the branch's physical
    // appraisal is what actually sets the loan amount.
    String? goldOrnamentType,
    double? goldApproxWeightGrams,
    int? goldApproxPurityKarat,
    bool? goldAlreadyPledged,
    String? goldLoanBranch,
    // Business Loan and MSME Loan — msmeUdyamNumber only means anything
    // for MSME, left null for Business Loan.
    String? businessName,
    String? businessRegistrationType,
    String? businessAddress,
    int? businessYearsInOperation,
    double? businessMonthlyTurnover,
    String? businessGstNumber,
    String? msmeUdyamNumber,
    // Farmer Loan only.
    String? farmerLandRecordType,
    String? farmerLandRecordNumber,
    double? farmerLandAreaAcres,
    String? farmerLandLocation,
    String? farmerCropOrPurpose,
    String? farmerKccNumber,
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
      if (builderName != null) 'builderName': builderName,
      if (projectName != null) 'projectName': projectName,
      if (propertyAddress != null) 'propertyAddress': propertyAddress,
      if (agreementValue != null) 'agreementValue': agreementValue,
      if (constructionStage != null) 'constructionStage': constructionStage,
      if (goldOrnamentType != null) 'goldOrnamentType': goldOrnamentType,
      if (goldApproxWeightGrams != null) 'goldApproxWeightGrams': goldApproxWeightGrams,
      if (goldApproxPurityKarat != null) 'goldApproxPurityKarat': goldApproxPurityKarat,
      if (goldAlreadyPledged != null) 'goldAlreadyPledged': goldAlreadyPledged,
      if (goldLoanBranch != null) 'goldLoanBranch': goldLoanBranch,
      if (businessName != null) 'businessName': businessName,
      if (businessRegistrationType != null) 'businessRegistrationType': businessRegistrationType,
      if (businessAddress != null) 'businessAddress': businessAddress,
      if (businessYearsInOperation != null) 'businessYearsInOperation': businessYearsInOperation,
      if (businessMonthlyTurnover != null) 'businessMonthlyTurnover': businessMonthlyTurnover,
      if (businessGstNumber != null) 'businessGstNumber': businessGstNumber,
      if (msmeUdyamNumber != null) 'msmeUdyamNumber': msmeUdyamNumber,
      if (farmerLandRecordType != null) 'farmerLandRecordType': farmerLandRecordType,
      if (farmerLandRecordNumber != null) 'farmerLandRecordNumber': farmerLandRecordNumber,
      if (farmerLandAreaAcres != null) 'farmerLandAreaAcres': farmerLandAreaAcres,
      if (farmerLandLocation != null) 'farmerLandLocation': farmerLandLocation,
      if (farmerCropOrPurpose != null) 'farmerCropOrPurpose': farmerCropOrPurpose,
      if (farmerKccNumber != null) 'farmerKccNumber': farmerKccNumber,
    }, token: token);
    return LoanApplication.fromJson(res);
  }

  /// GET /auth/branches — physical locations for Gold Loan, where the
  /// collateral itself has to be brought in and appraised in person.
  static Future<List<Branch>> branches(String token) async {
    final res = await ApiClient.getList('/auth/branches', token: token);
    return res.map((e) => Branch.fromJson(e as Map<String, dynamic>)).toList();
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
