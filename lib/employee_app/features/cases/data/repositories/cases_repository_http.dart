import '../../../../core/network/api_client.dart';
import '../../domain/entities/case_entity.dart';
import '../../domain/repositories/cases_repository.dart';

/// HTTP-backed [CasesRepository] — talks to the Go backend's employee
/// verification-task endpoints (internal/loans/employee_handler.go):
///   GET  /employee/tasks/today
///   POST /employee/loans/:id/verify
class CasesRepositoryHttp implements CasesRepository {
  CasesRepositoryHttp(this._apiClient);

  final ApiClient _apiClient;

  /// The backend has no single-loan employee endpoint, so case-detail
  /// lookups are served from the most recent `tasks/today` listing.
  List<CaseEntity> _cache = const [];

  @override
  Future<List<CaseEntity>> fetchCases() async {
    final response = await _apiClient.dio.get('/employee/tasks/today');
    final list = (response.data as List).cast<Map<String, dynamic>>();
    _cache = list.map(_fromLoanJson).toList();
    return List.unmodifiable(_cache);
  }

  @override
  Future<CaseEntity> fetchCaseById(String id) async {
    final cached = _cache.where((c) => c.id == id).toList();
    if (cached.isNotEmpty) return cached.first;
    await fetchCases();
    return _cache.firstWhere((c) => c.id == id, orElse: () => throw StateError('Case $id not found'));
  }

  @override
  Future<List<CaseTimelineEvent>> fetchTimeline(String caseId) async {
    // The backend doesn't track a per-loan verification timeline — synthesize
    // a minimal, honest one from the fields it does return.
    final caseItem = await fetchCaseById(caseId);
    final events = <CaseTimelineEvent>[
      CaseTimelineEvent(
        title: 'Case Assigned',
        timestamp: caseItem.assignedDate,
        description: 'Loan verification assigned to field officer.',
      ),
    ];
    if (caseItem.status == CaseStatus.completed) {
      events.add(CaseTimelineEvent(
        title: 'Verification Submitted',
        timestamp: DateTime.now(),
        description: 'Verification report submitted for review.',
      ));
    }
    return events;
  }

  @override
  Future<CaseEntity> verifyLoan(
    String id, {
    required String findings,
    required String remarks,
    required String riskAssessment,
    required String recommendation,
  }) async {
    final response = await _apiClient.dio.post('/employee/loans/$id/verify', data: {
      'findings': findings,
      'remarks': remarks,
      'riskAssessment': riskAssessment,
      'recommendation': recommendation,
    });
    final updated = _fromLoanJson(response.data as Map<String, dynamic>);
    final index = _cache.indexWhere((c) => c.id == id);
    if (index != -1) {
      _cache = List.of(_cache)..[index] = updated;
    }
    return updated;
  }

  @override
  Future<void> uploadLoanDocument(
    String loanId, {
    required String docType,
    required String fileName,
    required String mimeType,
    required String dataBase64,
  }) async {
    await _apiClient.dio.post('/employee/loans/$loanId/documents', data: {
      'docType': docType,
      'fileName': fileName,
      'mimeType': mimeType,
      'dataBase64': dataBase64,
    });
  }

  CaseEntity _fromLoanJson(Map<String, dynamic> json) {
    final id = '${json['id']}';
    final createdAt = DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now();
    final amount = (json['amountRequested'] as num?)?.toDouble() ?? 0;
    final addressLine = json['addressLine'] as String? ?? '';
    final city = json['city'] as String? ?? '';
    final referencesJson = (json['references'] as List?)?.cast<Map<String, dynamic>>() ?? const [];
    return CaseEntity(
      id: id,
      caseNumber: 'LN-$id',
      customerName: json['applicantName'] as String? ?? 'Unknown',
      customerPhone: json['applicantPhone'] as String? ?? '',
      address: [addressLine, city].where((s) => s.isNotEmpty).join(', '),
      loanType: loanTypeFromBackendCategory(json['category'] as String? ?? ''),
      loanAmount: amount,
      priority: amount >= 1000000 ? CasePriority.high : (amount >= 300000 ? CasePriority.medium : CasePriority.low),
      status: caseStatusFromBackend(json['status'] as String? ?? 'assigned'),
      assignedDate: createdAt,
      dueDate: createdAt.add(const Duration(days: 2)),
      cibilScore: (json['cibilScore'] as num?)?.toInt(),
      references: referencesJson.map(_fromReferenceJson).toList(),
      timingPreference: json['timingPreference'] as String?,
      visitDate: DateTime.tryParse(json['visitDate'] as String? ?? ''),
      visitTimeSlot: json['visitTimeSlot'] as String?,
      category: json['category'] as String? ?? '',
    );
  }

  LoanReference _fromReferenceJson(Map<String, dynamic> json) {
    return LoanReference(
      id: '${json['id']}',
      name: json['name'] as String? ?? '',
      relation: json['relation'] as String? ?? '',
      phone: json['phone'] as String? ?? '',
      aadhaarMasked: json['aadhaarMasked'] as String? ?? '',
      panMasked: json['panMasked'] as String? ?? '',
    );
  }
}
