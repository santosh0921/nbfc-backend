import '../entities/case_entity.dart';

abstract class CasesRepository {
  Future<List<CaseEntity>> fetchCases();
  Future<CaseEntity> fetchCaseById(String id);
  Future<List<CaseTimelineEvent>> fetchTimeline(String caseId);

  /// Submits a verification report for the loan, transitioning it to
  /// `verified` — `POST /employee/loans/:id/verify`.
  Future<CaseEntity> verifyLoan(
    String id, {
    required String findings,
    required String remarks,
    required String riskAssessment,
    required String recommendation,
  });

  /// Uploads a document/photo captured during a verification visit so it
  /// becomes visible to admin — `POST /employee/loans/:id/documents`.
  Future<void> uploadLoanDocument(
    String loanId, {
    required String docType,
    required String fileName,
    required String mimeType,
    required String dataBase64,
  });
}
