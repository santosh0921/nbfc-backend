/// A single support chat message as returned by the backend
/// (`internal/models/support.go` `SupportMessage`, serialized by
/// `GET /auth/support/messages` and `POST /auth/support/messages`). Field
/// names mirror the backend's JSON tags exactly (camelCase).
class SupportMessage {
  final int id;
  final int threadId;
  final String senderType; // "customer" | "admin"
  final String senderRef;
  final String body;
  final DateTime createdAt;

  const SupportMessage({
    required this.id,
    required this.threadId,
    required this.senderType,
    required this.senderRef,
    required this.body,
    required this.createdAt,
  });

  bool get isCustomer => senderType == 'customer';

  factory SupportMessage.fromJson(Map<String, dynamic> json) {
    return SupportMessage(
      id: json['id'] as int,
      threadId: json['threadId'] as int,
      senderType: json['senderType'] as String? ?? '',
      senderRef: json['senderRef'] as String? ?? '',
      body: json['body'] as String? ?? '',
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
    );
  }
}

/// A customer's support thread (`internal/models/support.go`
/// `SupportThread`). One open thread per customer at a time; the backend
/// auto-creates one on the customer's first message.
class SupportThread {
  final int id;
  final String customerId;
  final String status; // "open" | "closed"
  final DateTime createdAt;
  final DateTime updatedAt;

  const SupportThread({
    required this.id,
    required this.customerId,
    required this.status,
    required this.createdAt,
    required this.updatedAt,
  });

  factory SupportThread.fromJson(Map<String, dynamic> json) {
    return SupportThread(
      id: json['id'] as int,
      customerId: json['customerId'] as String? ?? '',
      status: json['status'] as String? ?? '',
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
      updatedAt: DateTime.tryParse(json['updatedAt'] as String? ?? '') ?? DateTime.now(),
    );
  }
}

/// Response shape of `GET /auth/support/messages`.
class SupportThreadResponse {
  final SupportThread? thread;
  final List<SupportMessage> messages;

  const SupportThreadResponse({required this.thread, required this.messages});
}
