import 'package:flutter/material.dart';
import '../../data/repositories/collection_repository.dart';
import '../../domain/entities/recovery_case_entity.dart';

class RecoveryDashboardProvider extends ChangeNotifier {
  RecoveryDashboardProvider(this._repository);

  final CollectionRepository _repository;

  bool isLoading = false;
  RecoveryDashboardSummary? summary;

  /// [silent] skips the isLoading flip so the periodic auto-refresh timer
  /// updates the KPI numbers without flashing the whole dashboard back to
  /// its loading spinner every cycle.
  Future<void> load({bool silent = false}) async {
    if (!silent) {
      isLoading = true;
      notifyListeners();
    }
    try {
      summary = await _repository.fetchDashboardSummary();
    } catch (_) {
      // Silent refresh failure just leaves the last-known summary showing.
    }
    if (!silent) isLoading = false;
    notifyListeners();
  }
}
