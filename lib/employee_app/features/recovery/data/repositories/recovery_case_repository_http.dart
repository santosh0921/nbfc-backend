import '../../../../core/network/api_client.dart';
import '../../domain/entities/recovery_case_entity.dart';
import 'recovery_case_repository.dart';

/// HTTP-backed [RecoveryCaseRepository] — talks to the Go backend's employee
/// recovery endpoints (internal/recovery/handler.go):
///   GET  /employee/recovery/cases/today
///   POST /employee/recovery/cases/:id/report
///
/// The backend has no per-action ("Contacted", "Escalate", ...) or timeline
/// endpoint for employees — those quick actions and the EMI history remain
/// local/synthesized, exactly as the mock repository behaved, so the
/// existing "Recovery Actions" UI keeps working unchanged.
class RecoveryCaseRepositoryHttp implements RecoveryCaseRepository {
  RecoveryCaseRepositoryHttp(this._apiClient);

  final ApiClient _apiClient;

  List<RecoveryCaseEntity> _cache = const [];
  final Map<String, List<RecoveryTimelineEvent>> _timelines = {};

  @override
  Future<List<RecoveryCaseEntity>> fetchCases() async {
    final response = await _apiClient.dio.get('/employee/recovery/cases/today');
    final list = (response.data as List).cast<Map<String, dynamic>>();
    _cache = list.map(_fromJson).toList();
    for (final c in _cache) {
      _timelines.putIfAbsent(
        c.id,
        () => [
          RecoveryTimelineEvent(
            title: 'Case Assigned',
            timestamp: c.assignedDate,
            description: 'Recovery case assigned for ${c.loanId}.',
          ),
        ],
      );
    }
    return List.unmodifiable(_cache);
  }

  @override
  Future<RecoveryCaseEntity> fetchCaseById(String id) async {
    final cached = _cache.where((c) => c.id == id).toList();
    if (cached.isNotEmpty) return cached.first;
    await fetchCases();
    return _cache.firstWhere((c) => c.id == id, orElse: () => throw StateError('Case $id not found'));
  }

  @override
  Future<List<RecoveryTimelineEvent>> fetchTimeline(String caseId) async {
    return List.unmodifiable(_timelines[caseId] ?? const []);
  }

  @override
  Future<List<EmiHistoryEntry>> fetchEmiHistory(String caseId) async {
    // The backend RecoveryCase model has no EMI schedule — synthesize a
    // plausible one from the outstanding amount, as the mock did.
    final caseItem = await fetchCaseById(caseId);
    final now = DateTime.now();
    final monthlyEmi = (caseItem.outstandingAmount / 12).clamp(1000, double.infinity);
    return List.generate(6, (i) {
      final dueDate = DateTime(now.year, now.month - (5 - i), 5);
      final overdue = i < caseItem.overdueEmiCount;
      return EmiHistoryEntry(
        month: _monthLabel(dueDate),
        dueDate: dueDate,
        amount: monthlyEmi.toDouble(),
        paid: !overdue,
        paidDate: overdue ? null : dueDate.add(const Duration(days: 2)),
      );
    });
  }

  @override
  Future<void> recordAction(String caseId, RecoveryStatus newStatus, RecoveryTimelineEvent event) async {
    final index = _cache.indexWhere((c) => c.id == caseId);
    if (index != -1) {
      _cache = List.of(_cache)..[index] = _copyWithStatus(_cache[index], newStatus);
    }
    _timelines.putIfAbsent(caseId, () => []).add(event);
  }

  @override
  Future<void> submitReport(
    String caseId, {
    required String statusUpdate,
    required String remarks,
    required String riskAssessment,
    required String recommendation,
    required String nextAction,
  }) async {
    await _apiClient.dio.post('/employee/recovery/cases/$caseId/report', data: {
      'statusUpdate': statusUpdate,
      'remarks': remarks,
      'riskAssessment': riskAssessment,
      'recommendation': recommendation,
      'nextAction': nextAction,
    });
  }

  RecoveryCaseEntity _fromJson(Map<String, dynamic> json) {
    final id = '${json['id']}';
    final createdAt = DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now();
    final overdueAmount = (json['overdueAmount'] as num?)?.toDouble() ?? 0;
    return RecoveryCaseEntity(
      id: id,
      loanId: json['loanRef'] as String? ?? id,
      customerName: json['customerName'] as String? ?? 'Unknown',
      customerPhone: json['customerPhone'] as String? ?? '',
      address: json['city'] as String? ?? '',
      loanType: LoanType.personal,
      outstandingAmount: overdueAmount,
      penaltyAmount: 0,
      interestAmount: 0,
      overdueEmiCount: overdueAmount > 0 ? 1 : 0,
      dpd: 0,
      priority: overdueAmount >= 200000 ? RecoveryPriority.high : (overdueAmount >= 50000 ? RecoveryPriority.medium : RecoveryPriority.low),
      riskLevel: overdueAmount >= 200000 ? RiskLevel.high : (overdueAmount >= 50000 ? RiskLevel.medium : RiskLevel.low),
      branch: json['city'] as String? ?? '',
      assignedDate: createdAt,
      status: _statusFromBackend(json['status'] as String? ?? 'open'),
    );
  }

  RecoveryStatus _statusFromBackend(String status) => switch (status) {
        'reported' => RecoveryStatus.visited,
        'resolved' => RecoveryStatus.closed,
        'escalated' => RecoveryStatus.escalated,
        _ => RecoveryStatus.assigned,
      };

  RecoveryCaseEntity _copyWithStatus(RecoveryCaseEntity c, RecoveryStatus status) => RecoveryCaseEntity(
        id: c.id,
        loanId: c.loanId,
        customerName: c.customerName,
        customerPhone: c.customerPhone,
        address: c.address,
        loanType: c.loanType,
        outstandingAmount: c.outstandingAmount,
        penaltyAmount: c.penaltyAmount,
        interestAmount: c.interestAmount,
        overdueEmiCount: c.overdueEmiCount,
        dpd: c.dpd,
        priority: c.priority,
        riskLevel: c.riskLevel,
        branch: c.branch,
        assignedDate: c.assignedDate,
        status: status,
      );

  static String _monthLabel(DateTime date) {
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    return '${months[date.month - 1]} ${date.year}';
  }
}
