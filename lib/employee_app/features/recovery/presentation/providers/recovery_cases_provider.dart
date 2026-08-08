import 'package:flutter/material.dart';
import '../../data/repositories/recovery_case_repository.dart';
import '../../domain/entities/recovery_case_entity.dart';

class RecoveryCasesProvider extends ChangeNotifier {
  RecoveryCasesProvider(this._repository);

  final RecoveryCaseRepository _repository;

  bool isLoading = false;
  String? errorMessage;
  List<RecoveryCaseEntity> _all = const [];

  String searchQuery = '';
  RecoveryPriority? priorityFilter;
  RiskLevel? riskFilter;
  RecoveryStatus? statusFilter;

  bool isLoadingDetail = false;
  String? detailErrorMessage;
  RecoveryCaseEntity? selectedCase;
  List<RecoveryTimelineEvent> timeline = const [];
  List<EmiHistoryEntry> emiHistory = const [];

  Future<void> load() async {
    isLoading = true;
    errorMessage = null;
    notifyListeners();
    try {
      _all = await _repository.fetchCases();
    } catch (e) {
      errorMessage = 'Could not load cases. Pull down to retry.';
    } finally {
      isLoading = false;
      notifyListeners();
    }
  }

  List<RecoveryCaseEntity> get filtered {
    return _all.where((c) {
      final matchesQuery = searchQuery.isEmpty ||
          c.customerName.toLowerCase().contains(searchQuery.toLowerCase()) ||
          c.loanId.toLowerCase().contains(searchQuery.toLowerCase());
      final matchesPriority = priorityFilter == null || c.priority == priorityFilter;
      final matchesRisk = riskFilter == null || c.riskLevel == riskFilter;
      final matchesStatus = statusFilter == null || c.status == statusFilter;
      return matchesQuery && matchesPriority && matchesRisk && matchesStatus;
    }).toList();
  }

  void setSearchQuery(String value) {
    searchQuery = value;
    notifyListeners();
  }

  void setPriorityFilter(RecoveryPriority? value) {
    priorityFilter = value;
    notifyListeners();
  }

  void setRiskFilter(RiskLevel? value) {
    riskFilter = value;
    notifyListeners();
  }

  void setStatusFilter(RecoveryStatus? value) {
    statusFilter = value;
    notifyListeners();
  }

  void clearFilters() {
    priorityFilter = null;
    riskFilter = null;
    statusFilter = null;
    notifyListeners();
  }

  void setPendingVisitsQuickFilter() {
    clearFilters();
    statusFilter = RecoveryStatus.visitScheduled;
    notifyListeners();
  }

  void setPromiseToPayQuickFilter() {
    clearFilters();
    statusFilter = RecoveryStatus.promiseToPay;
    notifyListeners();
  }

  void setClosedQuickFilter() {
    clearFilters();
    statusFilter = RecoveryStatus.closed;
    notifyListeners();
  }

  void _replaceInAll(RecoveryCaseEntity updated) {
    final index = _all.indexWhere((c) => c.id == updated.id);
    if (index != -1) {
      _all = List.of(_all)..[index] = updated;
    }
  }

  Future<void> loadCaseDetail(String id) async {
    isLoadingDetail = true;
    detailErrorMessage = null;
    notifyListeners();
    try {
      final results = await Future.wait([
        _repository.fetchCaseById(id),
        _repository.fetchTimeline(id),
        _repository.fetchEmiHistory(id),
      ]);
      selectedCase = results[0] as RecoveryCaseEntity;
      timeline = results[1] as List<RecoveryTimelineEvent>;
      emiHistory = results[2] as List<EmiHistoryEntry>;
      _replaceInAll(selectedCase!);
    } catch (e) {
      detailErrorMessage = 'Could not load case details. Please go back and retry.';
    } finally {
      isLoadingDetail = false;
      notifyListeners();
    }
  }

  Future<void> performAction(
    String caseId,
    RecoveryStatus newStatus,
    String actionLabel, {
    String? note,
  }) async {
    final event = RecoveryTimelineEvent(
      title: actionLabel,
      timestamp: DateTime.now(),
      description: note?.isNotEmpty == true ? note! : '$actionLabel logged for this case.',
    );
    await _repository.recordAction(caseId, newStatus, event);
    await loadCaseDetail(caseId);
  }

  /// Submits the full recovery report to the backend
  /// (`POST /employee/recovery/cases/:id/report`), then logs the same
  /// action locally so the on-screen timeline/status reflect it immediately.
  Future<void> submitReport(
    String caseId, {
    required String statusUpdate,
    required String remarks,
    required String riskAssessment,
    required String recommendation,
    required String nextAction,
    required RecoveryStatus newStatus,
    required String actionLabel,
  }) async {
    await _repository.submitReport(
      caseId,
      statusUpdate: statusUpdate,
      remarks: remarks,
      riskAssessment: riskAssessment,
      recommendation: recommendation,
      nextAction: nextAction,
    );
    await performAction(caseId, newStatus, actionLabel, note: remarks);
  }
}
