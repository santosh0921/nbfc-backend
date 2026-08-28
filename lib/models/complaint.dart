class Complaint {
  final int id;
  final String category;
  final String description;
  final String status;
  final String? adminNote;
  final DateTime createdAt;
  final DateTime? resolvedAt;

  const Complaint({
    required this.id,
    required this.category,
    required this.description,
    required this.status,
    this.adminNote,
    required this.createdAt,
    this.resolvedAt,
  });

  factory Complaint.fromJson(Map<String, dynamic> json) => Complaint(
        id: json['id'] as int,
        category: json['category'] as String? ?? '',
        description: json['description'] as String? ?? '',
        status: json['status'] as String? ?? 'open',
        adminNote: json['adminNote'] as String?,
        createdAt: DateTime.tryParse(json['createdAt'] as String? ?? '') ?? DateTime.now(),
        resolvedAt: json['resolvedAt'] != null ? DateTime.tryParse(json['resolvedAt'] as String) : null,
      );

  String get displayId => 'TCK-${id.toString().padLeft(5, '0')}';

  String get statusLabel => switch (status) {
        'open' => 'Open',
        'in_progress' => 'In Progress',
        'resolved' => 'Resolved',
        'closed' => 'Closed',
        _ => status,
      };
}
