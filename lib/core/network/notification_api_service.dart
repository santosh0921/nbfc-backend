import 'api_client.dart';
import '../../models/backend_notification.dart';

/// Wraps the backend's dual-auth `/notifications` endpoints
/// (`internal/notifications/handler.go`) — works with the customer's
/// existing Bearer JWT (the same token used for `/auth/*`), since the
/// route accepts either a customer or employee token and infers the
/// recipient from whichever validates.
class NotificationApiService {
  NotificationApiService._();

  static Future<List<BackendNotification>> list(String token) async {
    final res = await ApiClient.getList('/notifications', token: token);
    return res.map((e) => BackendNotification.fromJson(e as Map<String, dynamic>)).toList();
  }

  static Future<void> markRead(String token, int id) async {
    await ApiClient.post('/notifications/$id/read', const {}, token: token);
  }
}
