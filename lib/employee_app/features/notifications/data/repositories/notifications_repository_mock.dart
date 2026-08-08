import '../../domain/entities/notification_entity.dart';

class NotificationsRepositoryMock {
  static final _items = <NotificationEntity>[
    NotificationEntity(
      id: 'n-1',
      type: NotificationType.newAssignment,
      title: 'New Case Assigned',
      message: 'Personal Loan verification for Rahul Verma has been assigned to you.',
      timestamp: DateTime.now().subtract(const Duration(minutes: 20)),
      isRead: false,
    ),
    NotificationEntity(
      id: 'n-2',
      type: NotificationType.visitReminder,
      title: 'Visit Reminder',
      message: 'Site visit for Sunita Nair (Gold Loan) is scheduled at 12:00 PM today.',
      timestamp: DateTime.now().subtract(const Duration(hours: 1)),
      isRead: false,
    ),
    NotificationEntity(
      id: 'n-3',
      type: NotificationType.reportAccepted,
      title: 'Report Accepted',
      message: 'Your verification report for Priya Iyer has been approved.',
      timestamp: DateTime.now().subtract(const Duration(hours: 3)),
      isRead: true,
    ),
    NotificationEntity(
      id: 'n-4',
      type: NotificationType.caseUpdated,
      title: 'Case Updated',
      message: 'Additional documents requested for Deepak Kumar\'s case.',
      timestamp: DateTime.now().subtract(const Duration(hours: 6)),
      isRead: true,
    ),
    NotificationEntity(
      id: 'n-5',
      type: NotificationType.reportRejected,
      title: 'Report Rejected',
      message: 'Verification report for Meera Joshi was rejected — resubmission required.',
      timestamp: DateTime.now().subtract(const Duration(days: 1)),
      isRead: true,
    ),
  ];

  Future<List<NotificationEntity>> fetchAll() async {
    await Future.delayed(const Duration(milliseconds: 400));
    return List.unmodifiable(_items);
  }

  Future<void> markAsRead(String id) async {
    final index = _items.indexWhere((n) => n.id == id);
    if (index != -1) {
      _items[index] = _items[index].copyWith(isRead: true);
    }
  }

  Future<void> markAllAsRead() async {
    for (var i = 0; i < _items.length; i++) {
      _items[i] = _items[i].copyWith(isRead: true);
    }
  }
}
