import '../../../../core/network/api_client.dart';
import '../../domain/entities/notification_entity.dart';

/// HTTP-backed notifications repository — talks to the dual-auth (customer
/// or employee JWT) notification endpoints (internal/notifications/handler.go):
///   GET  /notifications
///   POST /notifications/:id/read
class NotificationsRepositoryHttp {
  NotificationsRepositoryHttp(this._apiClient);

  final ApiClient _apiClient;

  Future<List<NotificationEntity>> fetchAll() async {
    final response = await _apiClient.dio.get('/notifications');
    final list = (response.data as List).cast<Map<String, dynamic>>();
    return list.map(_fromJson).toList();
  }

  Future<void> markAsRead(String id) async {
    await _apiClient.dio.post('/notifications/$id/read');
  }

  /// The backend has no bulk "mark all read" endpoint — mark each unread
  /// notification individually so the existing "Mark all as read" UI action
  /// keeps working.
  Future<void> markAllAsRead() async {
    final all = await fetchAll();
    for (final n in all.where((n) => !n.isRead)) {
      await markAsRead(n.id);
    }
  }

  NotificationEntity _fromJson(Map<String, dynamic> json) {
    return NotificationEntity(
      id: '${json['id']}',
      type: _typeFromTitle(json['title'] as String? ?? ''),
      title: json['title'] as String? ?? '',
      message: json['body'] as String? ?? '',
      timestamp: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
      isRead: json['read'] as bool? ?? false,
    );
  }

  /// The backend's `Notification` model (internal/models/notification.go)
  /// only has a free-text title/body, not a typed category — infer the
  /// closest [NotificationType] from the title so existing icon/badge
  /// styling in the UI still varies sensibly.
  NotificationType _typeFromTitle(String title) {
    final t = title.toLowerCase();
    if (t.contains('assign')) return NotificationType.newAssignment;
    if (t.contains('visit')) return NotificationType.visitReminder;
    if (t.contains('reject')) return NotificationType.reportRejected;
    if (t.contains('accept') || t.contains('approve')) return NotificationType.reportAccepted;
    return NotificationType.caseUpdated;
  }
}
