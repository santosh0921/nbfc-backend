import '../../domain/entities/case_entity.dart';

/// Mock repository — swap internals for a Dio-backed remote datasource once
/// the Go `/employee/cases` endpoint exists; provider/UI layers untouched.
class CasesRepositoryMock {
  static final _cases = <CaseEntity>[
    CaseEntity(
      id: 'case-1',
      caseNumber: 'OF-CN-10231',
      customerName: 'Rahul Verma',
      customerPhone: '+91 98200 11223',
      address: '14B, Koregaon Park, Pune, MH 411001',
      loanType: LoanType.personal,
      loanAmount: 500000,
      priority: CasePriority.high,
      status: CaseStatus.assigned,
      assignedDate: DateTime.now().subtract(const Duration(days: 1)),
      dueDate: DateTime.now().add(const Duration(days: 1)),
    ),
    CaseEntity(
      id: 'case-2',
      caseNumber: 'OF-CN-10232',
      customerName: 'Sunita Nair',
      customerPhone: '+91 90040 55667',
      address: '22, MG Road, Bengaluru, KA 560001',
      loanType: LoanType.gold,
      loanAmount: 150000,
      priority: CasePriority.medium,
      status: CaseStatus.inProgress,
      assignedDate: DateTime.now().subtract(const Duration(days: 2)),
      dueDate: DateTime.now().add(const Duration(days: 2)),
    ),
    CaseEntity(
      id: 'case-3',
      caseNumber: 'OF-CN-10233',
      customerName: 'Deepak Kumar',
      customerPhone: '+91 91234 66778',
      address: '7, Sector 21, Noida, UP 201301',
      loanType: LoanType.home,
      loanAmount: 3500000,
      priority: CasePriority.high,
      status: CaseStatus.pendingReview,
      assignedDate: DateTime.now().subtract(const Duration(days: 4)),
      dueDate: DateTime.now().add(const Duration(hours: 6)),
    ),
    CaseEntity(
      id: 'case-4',
      caseNumber: 'OF-CN-10234',
      customerName: 'Priya Iyer',
      customerPhone: '+91 89990 33445',
      address: '5th Cross, Anna Nagar, Chennai, TN 600040',
      loanType: LoanType.business,
      loanAmount: 900000,
      priority: CasePriority.low,
      status: CaseStatus.completed,
      assignedDate: DateTime.now().subtract(const Duration(days: 6)),
      dueDate: DateTime.now().subtract(const Duration(days: 1)),
    ),
    CaseEntity(
      id: 'case-5',
      caseNumber: 'OF-CN-10235',
      customerName: 'Arjun Rathore',
      customerPhone: '+91 99887 22110',
      address: '18, Vaishali Nagar, Jaipur, RJ 302021',
      loanType: LoanType.vehicle,
      loanAmount: 650000,
      priority: CasePriority.medium,
      status: CaseStatus.assigned,
      assignedDate: DateTime.now(),
      dueDate: DateTime.now().add(const Duration(days: 3)),
    ),
    CaseEntity(
      id: 'case-6',
      caseNumber: 'OF-CN-10236',
      customerName: 'Meera Joshi',
      customerPhone: '+91 97123 44556',
      address: '3, Model Town, Ludhiana, PB 141002',
      loanType: LoanType.msme,
      loanAmount: 1200000,
      priority: CasePriority.high,
      status: CaseStatus.rejected,
      assignedDate: DateTime.now().subtract(const Duration(days: 8)),
      dueDate: DateTime.now().subtract(const Duration(days: 3)),
    ),
  ];

  Future<List<CaseEntity>> fetchCases() async {
    await Future.delayed(const Duration(milliseconds: 450));
    return List.unmodifiable(_cases);
  }

  Future<CaseEntity> fetchCaseById(String id) async {
    await Future.delayed(const Duration(milliseconds: 300));
    return _cases.firstWhere((c) => c.id == id);
  }

  Future<List<CaseTimelineEvent>> fetchTimeline(String caseId) async {
    await Future.delayed(const Duration(milliseconds: 300));
    final now = DateTime.now();
    return [
      CaseTimelineEvent(title: 'Case Assigned', timestamp: now.subtract(const Duration(days: 2)), description: 'Case assigned to field officer.'),
      CaseTimelineEvent(title: 'Customer Contacted', timestamp: now.subtract(const Duration(days: 1, hours: 4)), description: 'Call completed, visit scheduled.'),
      CaseTimelineEvent(title: 'Site Visit Scheduled', timestamp: now.subtract(const Duration(hours: 20)), description: 'Visit scheduled for verification.'),
    ];
  }
}
