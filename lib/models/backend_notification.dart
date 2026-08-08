/// A notification as returned by the backend (`internal/models/notification.go`),
/// served via the dual-auth `GET /notifications` endpoint. Mutable `read`
/// flag so the list UI can optimistically flip it on tap.
class BackendNotification {
  BackendNotification({
    required this.id,
    required this.title,
    required this.body,
    required this.read,
    required this.createdAt,
  });

  final int id;
  final String title;
  final String body;
  bool read;
  final DateTime createdAt;

  factory BackendNotification.fromJson(Map<String, dynamic> json) {
    return BackendNotification(
      id: json['id'] as int,
      title: json['title'] as String? ?? '',
      body: json['body'] as String? ?? '',
      read: json['read'] as bool? ?? false,
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
    );
  }
}
