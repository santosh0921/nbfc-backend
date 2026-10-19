import 'package:flutter/material.dart';
import '../../data/repositories/dashboard_repository_http.dart';
import '../../domain/entities/dashboard_summary.dart';

class DashboardProvider extends ChangeNotifier {
  DashboardProvider(this._repository);

  final DashboardRepositoryHttp _repository;

  bool isLoading = false;
  List<TaskItem> tasks = const [];
  TaskStatus? filter;

  int get completedCount => tasks.where((t) => t.status == TaskStatus.completed).length;
  int get remainingCount => tasks.where((t) => t.status == TaskStatus.remaining).length;
  int get postponedCount => tasks.where((t) => t.status == TaskStatus.postponed).length;

  List<TaskItem> get filteredTasks => filter == null ? tasks : tasks.where((t) => t.status == filter).toList();

  /// [silent] skips the isLoading flip (no full-screen spinner) — used by
  /// the periodic auto-refresh timer so new/updated cases appear as data
  /// changes underneath the user without the whole screen flashing back
  /// to a loading state every 15s. The very first load (from initState)
  /// should still show the spinner, so it stays the default.
  Future<void> load({bool silent = false}) async {
    if (!silent) {
      isLoading = true;
      notifyListeners();
    }
    try {
      tasks = await _repository.fetchTodaysTasks();
    } catch (_) {
      // A silent background refresh that fails just leaves the last-known
      // data on screen — no error UI needed for a timer tick nobody asked
      // for. The initial (non-silent) load still surfaces failure via
      // isLoading resolving to false with empty tasks, same as before.
    }
    if (!silent) isLoading = false;
    notifyListeners();
  }

  void setFilter(TaskStatus? value) {
    filter = filter == value ? null : value;
    notifyListeners();
  }
}
