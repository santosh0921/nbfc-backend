import 'package:equatable/equatable.dart';

enum TaskStatus { completed, remaining, postponed }

class TaskItem extends Equatable {
  const TaskItem({
    required this.id,
    required this.title,
    required this.customerName,
    required this.customerPhone,
    required this.date,
    required this.time,
    required this.priority,
    required this.status,
  });

  final String id;
  final String title;
  final String customerName;
  final String customerPhone;
  final DateTime date;
  final String time;
  final String priority;
  final TaskStatus status;

  @override
  List<Object?> get props => [id, title, customerName, customerPhone, date, time, priority, status];
}
