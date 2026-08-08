import 'package:flutter/material.dart';
import '../../domain/entities/recovery_case_entity.dart';

class RecoveryReportProvider extends ChangeNotifier {
  RecoveryStatus status = RecoveryStatus.contacted;
  RiskLevel riskAssessment = RiskLevel.medium;
  RecoveryPriority priority = RecoveryPriority.medium;
  String remarks = '';
  String recommendation = '';
  DateTime nextAction = DateTime.now().add(const Duration(days: 3));
  final List<String> photoPaths = [];
  final List<String> documentPaths = [];
  bool isSubmitting = false;

  void reset() {
    status = RecoveryStatus.contacted;
    riskAssessment = RiskLevel.medium;
    priority = RecoveryPriority.medium;
    remarks = '';
    recommendation = '';
    nextAction = DateTime.now().add(const Duration(days: 3));
    photoPaths.clear();
    documentPaths.clear();
    isSubmitting = false;
  }

  void setStatus(RecoveryStatus value) {
    status = value;
    notifyListeners();
  }

  void setRiskAssessment(RiskLevel value) {
    riskAssessment = value;
    notifyListeners();
  }

  void setPriority(RecoveryPriority value) {
    priority = value;
    notifyListeners();
  }

  void setRemarks(String value) {
    remarks = value;
  }

  void setRecommendation(String value) {
    recommendation = value;
  }

  void setNextAction(DateTime value) {
    nextAction = value;
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

  RecoveryReport build() {
    return RecoveryReport(
      status: status,
      remarks: remarks,
      riskAssessment: riskAssessment,
      recommendation: recommendation,
      nextAction: nextAction,
      priority: priority,
      photoPaths: List.of(photoPaths),
      documentPaths: List.of(documentPaths),
      submittedAt: DateTime.now(),
    );
  }
}
