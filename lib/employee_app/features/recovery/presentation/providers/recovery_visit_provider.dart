import 'package:flutter/material.dart';
import '../../data/repositories/payment_repository.dart';
import '../../data/repositories/visit_repository.dart';
import '../../domain/entities/recovery_case_entity.dart';
import '../utils/recovery_pdf_generator.dart';

class RecoveryVisitProvider extends ChangeNotifier {
  RecoveryVisitProvider(this._visitRepository, this._paymentRepository);

  final VisitRepository _visitRepository;
  final PaymentRepository _paymentRepository;

  double? latitude;
  double? longitude;
  DateTime? checkInTime;
  bool customerPresent = true;
  String notes = '';
  final List<String> photoPaths = [];
  final List<String> documentPaths = [];
  String? signaturePath;
  bool isSubmitting = false;

  void reset() {
    latitude = null;
    longitude = null;
    checkInTime = null;
    customerPresent = true;
    notes = '';
    photoPaths.clear();
    documentPaths.clear();
    signaturePath = null;
    isSubmitting = false;
  }

  void setCheckIn(double lat, double lng) {
    latitude = lat;
    longitude = lng;
    checkInTime = DateTime.now();
    notifyListeners();
  }

  void setCustomerPresent(bool value) {
    customerPresent = value;
    notifyListeners();
  }

  void setNotes(String value) {
    notes = value;
    notifyListeners();
  }

  void addPhoto(String path) {
    photoPaths.add(path);
    notifyListeners();
  }

  void addDocument(String path) {
    documentPaths.add(path);
    notifyListeners();
  }

  void setSignature(String path) {
    signaturePath = path;
    notifyListeners();
  }

  Future<VisitRecord> submitVisit(String caseId, RecoveryCaseEntity caseItem) async {
    isSubmitting = true;
    notifyListeners();
    var record = VisitRecord(
      checkInTime: checkInTime ?? DateTime.now(),
      latitude: latitude ?? 0,
      longitude: longitude ?? 0,
      customerPresent: customerPresent,
      notes: notes,
      photoPaths: List.of(photoPaths),
      documentPaths: List.of(documentPaths),
      signaturePath: signaturePath,
    );
    try {
      final pdfFile = await RecoveryPdfGenerator.generateVisitReport(caseItem, record);
      record = record.copyWith(pdfPath: pdfFile.path);
    } catch (_) {
      // PDF generation is best-effort; the visit is still recorded without it.
    }
    await _visitRepository.submitVisit(caseId, record);
    isSubmitting = false;
    notifyListeners();
    return record;
  }

  Future<PaymentRecord> recordPayment({
    required String caseId,
    required double amount,
    required PaymentMode mode,
    required String remarks,
  }) {
    return _paymentRepository.recordPayment(caseId: caseId, amount: amount, mode: mode, remarks: remarks);
  }
}
