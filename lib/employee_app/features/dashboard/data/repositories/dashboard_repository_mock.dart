import '../../domain/entities/dashboard_summary.dart';

/// Mock repository — replace internals with a Dio-backed remote datasource
/// call once the Go `/employee/tasks` endpoint exists. The provider
/// consuming this does not need to change.
class DashboardRepositoryMock {
  Future<List<TaskItem>> fetchTodaysTasks() async {
    await Future.delayed(const Duration(milliseconds: 450));
    final today = DateTime.now();
    return [
      TaskItem(
        id: 'task-1',
        title: 'Personal Loan Verification',
        customerName: 'Rahul Verma',
        customerPhone: '+91 98200 11223',
        date: today,
        time: '10:30 AM',
        priority: 'High',
        status: TaskStatus.remaining,
      ),
      TaskItem(
        id: 'task-2',
        title: 'Gold Loan Appraisal',
        customerName: 'Sunita Nair',
        customerPhone: '+91 90040 55667',
        date: today,
        time: '12:00 PM',
        priority: 'Medium',
        status: TaskStatus.remaining,
      ),
      TaskItem(
        id: 'task-3',
        title: 'EMI Collection',
        customerName: 'Deepak Kumar',
        customerPhone: '+91 91234 66778',
        date: today,
        time: '3:00 PM',
        priority: 'High',
        status: TaskStatus.completed,
      ),
      TaskItem(
        id: 'task-4',
        title: 'Home Loan Property Check',
        customerName: 'Priya Iyer',
        customerPhone: '+91 89990 33445',
        date: today,
        time: '9:00 AM',
        priority: 'Medium',
        status: TaskStatus.completed,
      ),
      TaskItem(
        id: 'task-5',
        title: 'Business Loan Document Pickup',
        customerName: 'Arjun Rathore',
        customerPhone: '+91 99887 22110',
        date: today,
        time: '1:30 PM',
        priority: 'Low',
        status: TaskStatus.postponed,
      ),
      TaskItem(
        id: 'task-6',
        title: 'Vehicle Loan Video KYC',
        customerName: 'Meera Joshi',
        customerPhone: '+91 97123 44556',
        date: today,
        time: '4:30 PM',
        priority: 'Medium',
        status: TaskStatus.postponed,
      ),
    ];
  }
}
