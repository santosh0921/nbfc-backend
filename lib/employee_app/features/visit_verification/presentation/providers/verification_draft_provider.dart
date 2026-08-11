import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:uuid/uuid.dart';
import '../../../../core/utils/pdf_generator.dart';
import '../../../cases/domain/repositories/cases_repository.dart';
import '../../data/repositories/visit_history_repository.dart';
import '../../domain/entities/visit_record.dart';

/// Holds the working state of a single in-progress verification visit —
/// customer details, witnesses, captured documents, the geotagged site
/// photo, and both signatures — from the moment the officer starts a visit
/// until it's submitted and persisted to history (locally, for the PDF/visit
/// trail) and reported to the backend via `POST /employee/loans/:id/verify`.
class VerificationDraftProvider extends ChangeNotifier {
  VerificationDraftProvider(this._historyRepository, this._casesRepository);

  final VisitHistoryRepository _historyRepository;
  final CasesRepository _casesRepository;

  String caseId = '';
  String customerName = '';
  String loanType = '';

  String occupation = '';
  String monthlyIncome = '';
  String remarks = '';
  List<WitnessInfo> witnesses = [];
  List<CapturedDocument> documents = [];
  GeoPhoto? geoPhoto;
  String? customerSignaturePath;
  String? employeeSignaturePath;
  String recommendation = 'Approve for Review';
  String riskAssessment = 'Low';

  bool isSubmitting = false;
  VisitRecord? lastSubmitted;

  void startVisit({required String caseId, required String customerName, required String loanType}) {
    this.caseId = caseId;
    this.customerName = customerName;
    this.loanType = loanType;
    occupation = '';
    monthlyIncome = '';
    remarks = '';
    witnesses = [];
    documents = [];
    geoPhoto = null;
    customerSignaturePath = null;
    employeeSignaturePath = null;
    recommendation = 'Approve for Review';
    riskAssessment = 'Low';
    lastSubmitted = null;
    notifyListeners();
  }

  void updateDetails({required String occupation, required String monthlyIncome, required String remarks}) {
    this.occupation = occupation;
    this.monthlyIncome = monthlyIncome;
    this.remarks = remarks;
    notifyListeners();
  }

  void addWitness(WitnessInfo witness) {
    witnesses = [...witnesses, witness];
    notifyListeners();
  }

  void removeWitness(int index) {
    witnesses = [...witnesses]..removeAt(index);
    notifyListeners();
  }

  /// Attaches a captured ID document photo to the witness at [index],
  /// both locally (so the review screen and PDF can show it) and to the
  /// backend so it appears in the admin panel like every other
  /// verification document. Upload failure is non-blocking — the local
  /// attachment is the source of truth for the on-device flow.
  Future<void> addWitnessDocument(int index, String filePath) async {
    if (index < 0 || index >= witnesses.length) return;
    final updated = [...witnesses];
    updated[index] = updated[index].copyWith(documentPath: filePath);
    witnesses = updated;
    notifyListeners();

    try {
      final bytes = await File(filePath).readAsBytes();
      await _casesRepository.uploadLoanDocument(
        caseId,
        docType: 'witness_${index + 1}_id',
        fileName: filePath.split(Platform.pathSeparator).last,
        mimeType: 'image/jpeg',
        dataBase64: base64Encode(bytes),
      );
    } catch (_) {
      // Non-fatal — same pattern as the checklist document upload below;
      // the local copy still exists and still goes into the PDF.
    }
  }

  void addDocument(CapturedDocument document) {
    documents = [...documents.where((d) => d.type != document.type), document];
    notifyListeners();
  }

  void setGeoPhoto(GeoPhoto photo) {
    geoPhoto = photo;
    notifyListeners();
  }

  void setCustomerSignature(String path) {
    customerSignaturePath = path;
    notifyListeners();
  }

  void setEmployeeSignature(String path) {
    employeeSignaturePath = path;
    notifyListeners();
  }

  void setRecommendation(String value) {
    recommendation = value;
    notifyListeners();
  }

  void setRiskAssessment(String value) {
    riskAssessment = value;
    notifyListeners();
  }

  Future<VisitRecord> submit() async {
    isSubmitting = true;
    notifyListeners();

    var record = VisitRecord(
      id: const Uuid().v4(),
      caseId: caseId,
      customerName: customerName,
      loanType: loanType,
      submittedAt: DateTime.now(),
      occupation: occupation,
      monthlyIncome: monthlyIncome,
      remarks: remarks,
      witnesses: witnesses,
      documents: documents,
      geoPhoto: geoPhoto,
      customerSignaturePath: customerSignaturePath,
      employeeSignaturePath: employeeSignaturePath,
      recommendation: recommendation,
    );

    final pdfFile = await PdfGenerator.generateVisitReport(record);
    record = record.copyWith(pdfPath: pdfFile.path);

    await _historyRepository.save(record);

    // Report the verification result to the backend, transitioning the loan
    // to `verified` (internal/loans/employee_handler.go). `caseId` here is
    // the backend's numeric loan id, threaded in via VerificationFlowPage.
    await _casesRepository.verifyLoan(
      caseId,
      findings: 'Occupation: $occupation. Monthly income: ₹$monthlyIncome.',
      remarks: remarks,
      riskAssessment: riskAssessment,
      recommendation: recommendation,
    );

    lastSubmitted = record;
    isSubmitting = false;
    notifyListeners();
    return record;
  }
}
