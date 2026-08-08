import 'package:equatable/equatable.dart';

enum CollectionStatus { pending, collected, overdue }

class CollectionEntity extends Equatable {
  const CollectionEntity({
    required this.id,
    required this.customerName,
    required this.loanAccountNumber,
    required this.emiAmount,
    required this.dueDate,
    required this.status,
  });

  final String id;
  final String customerName;
  final String loanAccountNumber;
  final double emiAmount;
  final DateTime dueDate;
  final CollectionStatus status;

  @override
  List<Object?> get props => [id, customerName, loanAccountNumber, emiAmount, dueDate, status];
}
