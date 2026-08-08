import 'package:flutter/material.dart';
import '../../data/repositories/collection_repository.dart';
import '../../domain/entities/recovery_case_entity.dart';

class RecoveryDashboardProvider extends ChangeNotifier {
  RecoveryDashboardProvider(this._repository);

  final CollectionRepository _repository;

  bool isLoading = false;
  RecoveryDashboardSummary? summary;

  Future<void> load() async {
    isLoading = true;
    notifyListeners();
    summary = await _repository.fetchDashboardSummary();
    isLoading = false;
    notifyListeners();
  }
}
