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
