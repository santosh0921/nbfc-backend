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
}
