import 'api_client.dart';
import '../../models/support_message.dart';

/// Wraps the backend's protected `/auth/support/messages` endpoints
/// (`internal/support/handler.go`) — posting a customer support message
/// (auto-creating a thread if none is open) and reading the logged-in
/// customer's own thread + full message history back.
class SupportApiService {
  SupportApiService._();

  static Future<SupportMessage> send(String token, String body) async {
    final res = await ApiClient.post('/auth/support/messages', {'body': body}, token: token);
    return SupportMessage.fromJson(res);
  }

  static Future<SupportThreadResponse> list(String token) async {
    final res = await ApiClient.get('/auth/support/messages', token: token);
    final threadJson = res['thread'];
    final thread = threadJson == null ? null : SupportThread.fromJson(threadJson as Map<String, dynamic>);
    final messages = (res['messages'] as List<dynamic>? ?? [])
        .map((e) => SupportMessage.fromJson(e as Map<String, dynamic>))
        .toList();
    return SupportThreadResponse(thread: thread, messages: messages);
  }
}
