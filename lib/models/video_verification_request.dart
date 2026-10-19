/// A KYC officer's request for the customer to complete a video
/// verification call, as returned by the backend's
/// `GET /kyc/verification-requests` / `POST .../:id/respond` endpoints.
class VideoVerificationRequest {
  VideoVerificationRequest({
    required this.id,
    required this.customerId,
    required this.requestedByAdminId,
    required this.requestedByAdminName,
    required this.status,
    required this.preferredTime,
    required this.respondedAt,
    required this.createdAt,
    required this.updatedAt,
  });

  final int id;
  final String customerId;
  final int requestedByAdminId;
  final String requestedByAdminName;

  /// "pending" | "customer_ready" | "declined"
  final String status;
  final String preferredTime;
  final DateTime? respondedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  bool get isPending => status == 'pending';

  factory VideoVerificationRequest.fromJson(Map<String, dynamic> json) {
    return VideoVerificationRequest(
      id: json['id'] as int,
      customerId: json['customerId'] as String? ?? '',
      requestedByAdminId: json['requestedByAdminId'] as int? ?? 0,
      requestedByAdminName: json['requestedByAdminName'] as String? ?? '',
      status: json['status'] as String? ?? 'pending',
      preferredTime: json['preferredTime'] as String? ?? '',
      respondedAt: json['respondedAt'] == null ? null : DateTime.tryParse(json['respondedAt'] as String),
      createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
      updatedAt: DateTime.tryParse(json['updatedAt'] as String? ?? '') ?? DateTime.now(),
    );
  }
}
