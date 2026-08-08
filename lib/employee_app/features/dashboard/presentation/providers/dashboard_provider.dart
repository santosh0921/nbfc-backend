import 'package:flutter/material.dart';
import '../../data/repositories/dashboard_repository_mock.dart';
import '../../domain/entities/dashboard_summary.dart';

class DashboardProvider extends ChangeNotifier {
  DashboardProvider(this._repository);

  final DashboardRepositoryMock _repository;

  bool isLoading = false;
  List<TaskItem> tasks = const [];
  TaskStatus? filter;

  int get completedCount => tasks.where((t) => t.status == TaskStatus.completed).length;
  int get remainingCount => tasks.where((t) => t.status == TaskStatus.remaining).length;
  int get postponedCount => tasks.where((t) => t.status == TaskStatus.postponed).length;

  List<TaskItem> get filteredTasks => filter == null ? tasks : tasks.where((t) => t.status == filter).toList();

  Future<void> load() async {
    isLoading = true;
    notifyListeners();
    tasks = await _repository.fetchTodaysTasks();
    isLoading = false;
    notifyListeners();
  }

  void setFilter(TaskStatus? value) {
    filter = filter == value ? null : value;
    notifyListeners();
  }
}
