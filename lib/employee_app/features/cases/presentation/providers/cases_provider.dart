import 'package:flutter/material.dart';
import '../../domain/entities/case_entity.dart';
import '../../domain/repositories/cases_repository.dart';

class CasesProvider extends ChangeNotifier {
  CasesProvider(this._repository);

  final CasesRepository _repository;

  bool isLoading = false;
  String? errorMessage;
  List<CaseEntity> _all = const [];

  String searchQuery = '';
  LoanType? loanTypeFilter;
  CasePriority? priorityFilter;
  CaseStatus? statusFilter;

  /// [silent] skips the isLoading flip so the periodic auto-refresh timer
  /// can pick up newly-assigned/updated cases without flashing the whole
  /// list back to its loading skeleton every cycle.
  Future<void> load({bool silent = false}) async {
    if (!silent) {
      isLoading = true;
      errorMessage = null;
      notifyListeners();
    }
    try {
      _all = await _repository.fetchCases();
      if (silent) errorMessage = null;
    } catch (e) {
      if (!silent) errorMessage = 'Could not load cases. Pull down to retry.';
      // A silent refresh failure just leaves the last-known list showing.
    } finally {
      if (!silent) isLoading = false;
      notifyListeners();
    }
  }

  /// Claims a pending_claim case. Returns a user-facing message on
  /// failure (most commonly: someone else claimed it first) instead of
  /// throwing, so the caller can show it directly in a snackbar; always
  /// reloads the full list afterward either way, since a claim attempt —
  /// win or lose — changes what should still be visible.
  Future<String?> claim(String id) async {
    try {
      await _repository.claimCase(id);
      return null;
    } catch (e) {
      return 'Could not claim this case — it may have already been claimed by another employee.';
    } finally {
      await load();
    }
  }

  List<CaseEntity> get filtered {
    return _all.where((c) {
      final matchesQuery = searchQuery.isEmpty ||
          c.customerName.toLowerCase().contains(searchQuery.toLowerCase()) ||
          c.caseNumber.toLowerCase().contains(searchQuery.toLowerCase());
      final matchesLoanType = loanTypeFilter == null || c.loanType == loanTypeFilter;
      final matchesPriority = priorityFilter == null || c.priority == priorityFilter;
      final matchesStatus = statusFilter == null || c.status == statusFilter;
      return matchesQuery && matchesLoanType && matchesPriority && matchesStatus;
    }).toList();
  }

  void setSearchQuery(String value) {
    searchQuery = value;
    notifyListeners();
  }

  void setLoanTypeFilter(LoanType? value) {
    loanTypeFilter = value;
    notifyListeners();
  }

  void setPriorityFilter(CasePriority? value) {
    priorityFilter = value;
    notifyListeners();
  }

  void setStatusFilter(CaseStatus? value) {
    statusFilter = value;
    notifyListeners();
  }

  void clearFilters() {
    loanTypeFilter = null;
    priorityFilter = null;
    statusFilter = null;
    notifyListeners();
  }
}
