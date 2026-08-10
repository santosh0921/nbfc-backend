import 'package:intl/intl.dart';

import '../../../cases/domain/entities/case_entity.dart';
import '../../../cases/domain/repositories/cases_repository.dart';
import '../../domain/entities/dashboard_summary.dart';

/// Replaces [DashboardRepositoryMock] — its doc comment promised a
/// Dio-backed remote datasource once the Go endpoint existed; it already
/// existed (`GET /employee/tasks/today`), this repository was simply never
/// wired up to it. The dashboard's "today's tasks" list is
/// really the same data as the Cases screen (both come from
/// `GET /employee/tasks/today`), so this reuses [CasesRepository] rather
/// than issuing a second, duplicate network call, and maps its
/// [CaseEntity]s onto the dashboard's [TaskItem] shape.
class DashboardRepositoryHttp {
  DashboardRepositoryHttp(this._casesRepository);

  final CasesRepository _casesRepository;

  Future<List<TaskItem>> fetchTodaysTasks() async {
    final cases = await _casesRepository.fetchCases();
    return cases.map(_fromCase).toList();
  }

  TaskItem _fromCase(CaseEntity c) {
    return TaskItem(
      id: c.id,
      title: '${c.loanType.label} Verification',
      customerName: c.customerName,
      customerPhone: c.customerPhone,
      date: c.assignedDate,
      time: DateFormat.jm().format(c.assignedDate),
      priority: c.priority.label,
      status: _statusFromCase(c.status),
    );
  }

  TaskStatus _statusFromCase(CaseStatus status) => switch (status) {
        CaseStatus.completed || CaseStatus.rejected => TaskStatus.completed,
        CaseStatus.assigned || CaseStatus.inProgress || CaseStatus.pendingReview => TaskStatus.remaining,
      };
}
