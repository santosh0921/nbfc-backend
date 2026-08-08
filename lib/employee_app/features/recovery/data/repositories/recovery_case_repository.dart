import '../../domain/entities/recovery_case_entity.dart';

/// Repository contract for recovery case CRUD — implement against the Go
/// `/employee/recovery/cases` endpoints once they exist; every provider in
/// this feature is written against this interface, not the mock directly.
abstract class RecoveryCaseRepository {
  Future<List<RecoveryCaseEntity>> fetchCases();
  Future<RecoveryCaseEntity> fetchCaseById(String id);
  Future<List<RecoveryTimelineEvent>> fetchTimeline(String caseId);
  Future<List<EmiHistoryEntry>> fetchEmiHistory(String caseId);
  Future<void> recordAction(String caseId, RecoveryStatus newStatus, RecoveryTimelineEvent event);

  /// Submits a full recovery report for the case to the backend —
  /// `POST /employee/recovery/cases/:id/report`.
  Future<void> submitReport(
    String caseId, {
    required String statusUpdate,
    required String remarks,
    required String riskAssessment,
    required String recommendation,
    required String nextAction,
  });
}

class RecoveryCaseRepositoryMock implements RecoveryCaseRepository {
  static final List<RecoveryCaseEntity> _cases = [
    RecoveryCaseEntity(
      id: 'rec-1',
      loanId: 'PL-4471203',
      customerName: 'Suresh Pillai',
      customerPhone: '+91 98450 11227',
      address: '12, Anna Salai, Chennai, TN 600002',
      loanType: LoanType.personal,
      outstandingAmount: 184500,
      penaltyAmount: 9200,
      interestAmount: 14100,
      overdueEmiCount: 4,
      dpd: 62,
      priority: RecoveryPriority.high,
      riskLevel: RiskLevel.high,
      branch: 'Chennai Central',
      assignedDate: DateTime.now().subtract(const Duration(days: 3)),
      status: RecoveryStatus.assigned,
    ),
    RecoveryCaseEntity(
      id: 'rec-2',
      loanId: 'GL-2205587',
      customerName: 'Anjali Deshmukh',
      customerPhone: '+91 90210 44551',
      address: '4th Floor, FC Road, Pune, MH 411005',
      loanType: LoanType.gold,
      outstandingAmount: 62000,
      penaltyAmount: 2100,
      interestAmount: 3400,
      overdueEmiCount: 2,
      dpd: 28,
      priority: RecoveryPriority.medium,
      riskLevel: RiskLevel.medium,
      branch: 'Pune FC Road',
      assignedDate: DateTime.now().subtract(const Duration(days: 1)),
      status: RecoveryStatus.contacted,
    ),
    RecoveryCaseEntity(
      id: 'rec-3',
      loanId: 'BL-3391044',
      customerName: 'Manoj Tiwari',
      customerPhone: '+91 89998 77213',
      address: 'Shop 9, Sarojini Nagar, New Delhi 110023',
      loanType: LoanType.business,
      outstandingAmount: 412000,
      penaltyAmount: 21000,
      interestAmount: 33500,
      overdueEmiCount: 6,
      dpd: 94,
      priority: RecoveryPriority.high,
      riskLevel: RiskLevel.critical,
      branch: 'Delhi South',
      assignedDate: DateTime.now().subtract(const Duration(days: 6)),
      status: RecoveryStatus.escalated,
    ),
    RecoveryCaseEntity(
      id: 'rec-4',
      loanId: 'VL-5561209',
      customerName: 'Kavitha Reddy',
      customerPhone: '+91 91009 22114',
      address: '22, Banjara Hills, Hyderabad, TG 500034',
      loanType: LoanType.vehicle,
      outstandingAmount: 98000,
      penaltyAmount: 3200,
      interestAmount: 5100,
      overdueEmiCount: 3,
      dpd: 41,
      priority: RecoveryPriority.medium,
      riskLevel: RiskLevel.medium,
      branch: 'Hyderabad Banjara Hills',
      assignedDate: DateTime.now().subtract(const Duration(days: 2)),
      status: RecoveryStatus.promiseToPay,
    ),
    RecoveryCaseEntity(
      id: 'rec-5',
      loanId: 'PL-7743301',
      customerName: 'Ramesh Gowda',
      customerPhone: '+91 99456 33210',
      address: '5th Cross, Jayanagar, Bengaluru, KA 560011',
      loanType: LoanType.personal,
      outstandingAmount: 55000,
      penaltyAmount: 1400,
      interestAmount: 2200,
      overdueEmiCount: 1,
      dpd: 14,
      priority: RecoveryPriority.low,
      riskLevel: RiskLevel.low,
      branch: 'Bengaluru Jayanagar',
      assignedDate: DateTime.now(),
      status: RecoveryStatus.visitScheduled,
    ),
    RecoveryCaseEntity(
      id: 'rec-6',
      loanId: 'HL-9982341',
      customerName: 'Farhan Sheikh',
      customerPhone: '+91 88770 12456',
      address: 'B-Wing, Andheri East, Mumbai, MH 400069',
      loanType: LoanType.home,
      outstandingAmount: 780000,
      penaltyAmount: 18000,
      interestAmount: 42000,
      overdueEmiCount: 5,
      dpd: 78,
      priority: RecoveryPriority.high,
      riskLevel: RiskLevel.high,
      branch: 'Mumbai Andheri',
      assignedDate: DateTime.now().subtract(const Duration(days: 5)),
      status: RecoveryStatus.unableToLocate,
    ),
    RecoveryCaseEntity(
      id: 'rec-7',
      loanId: 'PL-1123890',
      customerName: 'Divya Menon',
      customerPhone: '+91 97891 22330',
      address: 'Vyttila, Kochi, KL 682019',
      loanType: LoanType.personal,
      outstandingAmount: 42000,
      penaltyAmount: 800,
      interestAmount: 1500,
      overdueEmiCount: 0,
      dpd: 0,
      priority: RecoveryPriority.low,
      riskLevel: RiskLevel.low,
      branch: 'Kochi Vyttila',
      assignedDate: DateTime.now().subtract(const Duration(days: 10)),
      status: RecoveryStatus.closed,
    ),
    RecoveryCaseEntity(
      id: 'rec-8',
      loanId: 'MSME-661782',
      customerName: 'Prakash Bhatt',
      customerPhone: '+91 90567 88213',
      address: 'GIDC Estate, Ahmedabad, GJ 380015',
      loanType: LoanType.msme,
      outstandingAmount: 265000,
      penaltyAmount: 11500,
      interestAmount: 19800,
      overdueEmiCount: 3,
      dpd: 55,
      priority: RecoveryPriority.medium,
      riskLevel: RiskLevel.high,
      branch: 'Ahmedabad GIDC',
      assignedDate: DateTime.now().subtract(const Duration(days: 4)),
      status: RecoveryStatus.fraudSuspicion,
    ),
  ];

  static final Map<String, List<RecoveryTimelineEvent>> _timelines = {
    for (final c in _cases)
      c.id: [
        RecoveryTimelineEvent(
          title: 'Case Assigned',
          timestamp: c.assignedDate,
          description: 'Recovery case assigned for ${c.loanId}.',
        ),
      ],
  };

  @override
  Future<List<RecoveryCaseEntity>> fetchCases() async {
    await Future.delayed(const Duration(milliseconds: 450));
    return List.unmodifiable(_cases);
  }

  @override
  Future<RecoveryCaseEntity> fetchCaseById(String id) async {
    await Future.delayed(const Duration(milliseconds: 300));
    return _cases.firstWhere((c) => c.id == id);
  }

  @override
  Future<List<RecoveryTimelineEvent>> fetchTimeline(String caseId) async {
    await Future.delayed(const Duration(milliseconds: 300));
    return List.unmodifiable(_timelines[caseId] ?? const []);
  }

  @override
  Future<List<EmiHistoryEntry>> fetchEmiHistory(String caseId) async {
    await Future.delayed(const Duration(milliseconds: 300));
    final caseItem = _cases.firstWhere((c) => c.id == caseId);
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
  Future<void> submitReport(
    String caseId, {
    required String statusUpdate,
    required String remarks,
    required String riskAssessment,
    required String recommendation,
    required String nextAction,
  }) async {
    await Future.delayed(const Duration(milliseconds: 400));
  }

  @override
  Future<void> recordAction(String caseId, RecoveryStatus newStatus, RecoveryTimelineEvent event) async {
    await Future.delayed(const Duration(milliseconds: 400));
    final index = _cases.indexWhere((c) => c.id == caseId);
    if (index == -1) return;
    _cases[index] = RecoveryCaseEntity(
      id: _cases[index].id,
      loanId: _cases[index].loanId,
      customerName: _cases[index].customerName,
      customerPhone: _cases[index].customerPhone,
      address: _cases[index].address,
      loanType: _cases[index].loanType,
      outstandingAmount: _cases[index].outstandingAmount,
      penaltyAmount: _cases[index].penaltyAmount,
      interestAmount: _cases[index].interestAmount,
      overdueEmiCount: _cases[index].overdueEmiCount,
      dpd: _cases[index].dpd,
      priority: _cases[index].priority,
      riskLevel: _cases[index].riskLevel,
      branch: _cases[index].branch,
      assignedDate: _cases[index].assignedDate,
      status: newStatus,
    );
    _timelines.putIfAbsent(caseId, () => []).add(event);
  }

  static String _monthLabel(DateTime date) {
    const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
    return '${months[date.month - 1]} ${date.year}';
  }
}
