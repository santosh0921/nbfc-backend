import '../../domain/entities/recovery_case_entity.dart';
import 'recovery_case_repository.dart';

/// Repository contract for collection performance/dashboard aggregation and
/// promise-to-pay tracking — implement against the Go
/// `/employee/recovery/collections` endpoints once they exist.
abstract class CollectionRepository {
  Future<RecoveryDashboardSummary> fetchDashboardSummary();
  Future<PromiseToPay> recordPromiseToPay({
    required String caseId,
    required double promisedAmount,
    required DateTime promisedDate,
    required String remarks,
    required DateTime reminderDate,
  });
  Future<List<PromiseToPay>> fetchPromisesForCase(String caseId);
}

class CollectionRepositoryMock implements CollectionRepository {
  CollectionRepositoryMock(this._caseRepository);

  final RecoveryCaseRepository _caseRepository;
  static final Map<String, List<PromiseToPay>> _promises = {};

  @override
  Future<RecoveryDashboardSummary> fetchDashboardSummary() async {
    await Future.delayed(const Duration(milliseconds: 400));
    final cases = await _caseRepository.fetchCases();
    final today = DateTime.now();
    final todaysAssigned = cases.where((c) =>
        c.assignedDate.year == today.year && c.assignedDate.month == today.month && c.assignedDate.day == today.day).length;
    final pendingVisits = cases.where((c) => c.status == RecoveryStatus.visitScheduled).length;
    final overdueAmount = cases.fold<double>(0, (sum, c) => sum + c.outstandingAmount);
    final promiseToPay = cases.where((c) => c.status == RecoveryStatus.promiseToPay).length;
    final brokenPromise = _promises.values.expand((p) => p).where((p) => p.status == PromiseStatus.broken).length;
    final closed = cases.where((c) => c.status == RecoveryStatus.closed).length;
    final performance = cases.isEmpty ? 0.0 : (closed / cases.length) * 100;
    final collectionPercent = cases.isEmpty
        ? 0.0
        : (cases.where((c) => c.status == RecoveryStatus.fullPayment || c.status == RecoveryStatus.closed).length /
                cases.length) *
            100;
    return RecoveryDashboardSummary(
      todaysAssignedCount: todaysAssigned,
      pendingVisitsCount: pendingVisits,
      overdueAmount: overdueAmount,
      promiseToPayCount: promiseToPay,
      brokenPromiseCount: brokenPromise,
      closedCasesCount: closed,
      recoveryPerformancePercent: performance,
      collectionPercent: collectionPercent,
    );
  }

  @override
  Future<PromiseToPay> recordPromiseToPay({
    required String caseId,
    required double promisedAmount,
    required DateTime promisedDate,
    required String remarks,
    required DateTime reminderDate,
  }) async {
    await Future.delayed(const Duration(milliseconds: 400));
    final promise = PromiseToPay(
      promisedAmount: promisedAmount,
      promisedDate: promisedDate,
      remarks: remarks,
      reminderDate: reminderDate,
      status: PromiseStatus.pending,
    );
    _promises.putIfAbsent(caseId, () => []).add(promise);
    return promise;
  }

  @override
  Future<List<PromiseToPay>> fetchPromisesForCase(String caseId) async {
    await Future.delayed(const Duration(milliseconds: 250));
    return List.unmodifiable(_promises[caseId] ?? const []);
  }
}
