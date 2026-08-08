import '../../domain/entities/recovery_case_entity.dart';

/// Repository contract for field-visit check-ins — implement against the Go
/// `/employee/recovery/visits` endpoints once they exist.
abstract class VisitRepository {
  Future<void> submitVisit(String caseId, VisitRecord visit);
  Future<List<VisitRecord>> fetchVisitsForCase(String caseId);
}

class VisitRepositoryMock implements VisitRepository {
  static final Map<String, List<VisitRecord>> _visits = {};

  @override
  Future<void> submitVisit(String caseId, VisitRecord visit) async {
    await Future.delayed(const Duration(milliseconds: 500));
    _visits.putIfAbsent(caseId, () => []).add(visit);
  }

  @override
  Future<List<VisitRecord>> fetchVisitsForCase(String caseId) async {
    await Future.delayed(const Duration(milliseconds: 250));
    return List.unmodifiable(_visits[caseId] ?? const []);
  }
}
