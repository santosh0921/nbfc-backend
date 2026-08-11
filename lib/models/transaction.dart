enum TransactionType { emiPayment, disbursal, fee, refund, partPayment }

enum TransactionStatus { success, pending, failed }

class AppTransaction {
  final String id;
  final TransactionType type;
  final TransactionStatus status;
  final double amount;
  final DateTime date;
  final String description;
  final String loanId;
  final String referenceNumber;

  const AppTransaction({
    required this.id,
    required this.type,
    required this.status,
    required this.amount,
    required this.date,
    required this.description,
    required this.loanId,
    required this.referenceNumber,
  });

  /// Parses the backend's `GET /auth/transactions` response
  /// (`internal/loans/transactions_handler.go` `CustomerTransaction`) —
  /// only `disbursal`/`emiPayment` are ever sent by that endpoint today,
  /// but the switch is total so `fee`/`refund`/`partPayment` stay valid
  /// enum values without needing a backend-known default to fall back on.
  factory AppTransaction.fromJson(Map<String, dynamic> json) {
    final typeStr = json['type'] as String? ?? 'emiPayment';
    final statusStr = json['status'] as String? ?? 'success';
    return AppTransaction(
      id: json['id'] as String? ?? '',
      type: switch (typeStr) {
        'disbursal' => TransactionType.disbursal,
        'fee' => TransactionType.fee,
        'refund' => TransactionType.refund,
        'partPayment' => TransactionType.partPayment,
        _ => TransactionType.emiPayment,
      },
      status: switch (statusStr) {
        'pending' => TransactionStatus.pending,
        'failed' => TransactionStatus.failed,
        _ => TransactionStatus.success,
      },
      amount: (json['amount'] as num?)?.toDouble() ?? 0,
      date: DateTime.tryParse(json['date'] as String? ?? '') ?? DateTime.now(),
      description: json['description'] as String? ?? '',
      loanId: json['loanId'] as String? ?? '',
      referenceNumber: json['referenceNumber'] as String? ?? '',
    );
  }
}
