import 'package:equatable/equatable.dart';

enum NotificationType { newAssignment, visitReminder, caseUpdated, reportAccepted, reportRejected }

extension NotificationTypeX on NotificationType {
  String get label => switch (this) {
        NotificationType.newAssignment => 'New Assignment',
        NotificationType.visitReminder => 'Visit Reminder',
        NotificationType.caseUpdated => 'Case Updated',
        NotificationType.reportAccepted => 'Report Accepted',
        NotificationType.reportRejected => 'Report Rejected',
      };
}

class NotificationEntity extends Equatable {
  const NotificationEntity({
    required this.id,
    required this.type,
    required this.title,
    required this.message,
    required this.timestamp,
    required this.isRead,
  });

  final String id;
  final NotificationType type;
  final String title;
  final String message;
  final DateTime timestamp;
  final bool isRead;

  NotificationEntity copyWith({bool? isRead}) => NotificationEntity(
        id: id,
        type: type,
        title: title,
        message: message,
        timestamp: timestamp,
        isRead: isRead ?? this.isRead,
      );

  @override
  List<Object?> get props => [id, type, title, message, timestamp, isRead];
}
