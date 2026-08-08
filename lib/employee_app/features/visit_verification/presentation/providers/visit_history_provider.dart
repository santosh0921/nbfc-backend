import 'package:flutter/material.dart';
import '../../data/repositories/visit_history_repository.dart';
import '../../domain/entities/visit_record.dart';

class VisitHistoryProvider extends ChangeNotifier {
  VisitHistoryProvider(this._repository);

  final VisitHistoryRepository _repository;

  List<VisitRecord> records = const [];

  void load() {
    records = _repository.fetchAll();
    notifyListeners();
  }
}
