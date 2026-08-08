import 'package:equatable/equatable.dart';
import '../../../cases/domain/entities/case_entity.dart' show LoanType;

export '../../../cases/domain/entities/case_entity.dart' show LoanType, LoanTypeX;

enum RecoveryStatus {
  assigned,
  contacted,
  visitScheduled,
  visited,
  promiseToPay,
  partialPayment,
  fullPayment,
  settlement,
  escalated,
  legalRecommended,
  unableToLocate,
  fraudSuspicion,
  closed,
}

extension RecoveryStatusX on RecoveryStatus {
  String get label => switch (this) {
        RecoveryStatus.assigned => 'Assigned',
        RecoveryStatus.contacted => 'Contacted',
        RecoveryStatus.visitScheduled => 'Visit Scheduled',
        RecoveryStatus.visited => 'Visited',
        RecoveryStatus.promiseToPay => 'Promise To Pay',
        RecoveryStatus.partialPayment => 'Partial Payment',
        RecoveryStatus.fullPayment => 'Full Payment',
        RecoveryStatus.settlement => 'Settlement Discussion',
        RecoveryStatus.escalated => 'Escalated',
        RecoveryStatus.legalRecommended => 'Legal Recommendation',
        RecoveryStatus.unableToLocate => 'Unable To Locate',
        RecoveryStatus.fraudSuspicion => 'Fraud Suspicion',
        RecoveryStatus.closed => 'Case Closed',
      };
}

enum RecoveryPriority { high, medium, low }

extension RecoveryPriorityX on RecoveryPriority {
  String get label => switch (this) { RecoveryPriority.high => 'High', RecoveryPriority.medium => 'Medium', RecoveryPriority.low => 'Low' };
}

enum RiskLevel { low, medium, high, critical }

extension RiskLevelX on RiskLevel {
  String get label => switch (this) {
        RiskLevel.low => 'Low Risk',
        RiskLevel.medium => 'Medium Risk',
        RiskLevel.high => 'High Risk',
        RiskLevel.critical => 'Critical Risk',
      };
}

class RecoveryCaseEntity extends Equatable {
  const RecoveryCaseEntity({
    required this.id,
    required this.loanId,
    required this.customerName,
    required this.customerPhone,
    required this.address,
    required this.loanType,
    required this.outstandingAmount,
    required this.penaltyAmount,
    required this.interestAmount,
    required this.overdueEmiCount,
    required this.dpd,
    required this.priority,
    required this.riskLevel,
    required this.branch,
    required this.assignedDate,
    required this.status,
  });

  final String id;
  final String loanId;
  final String customerName;
  final String customerPhone;
  final String address;
  final LoanType loanType;
  final double outstandingAmount;
  final double penaltyAmount;
  final double interestAmount;
  final int overdueEmiCount;
  final int dpd;
  final RecoveryPriority priority;
  final RiskLevel riskLevel;
  final String branch;
  final DateTime assignedDate;
  final RecoveryStatus status;

  @override
  List<Object?> get props => [
        id,
        loanId,
        customerName,
        customerPhone,
        address,
        loanType,
        outstandingAmount,
        penaltyAmount,
        interestAmount,
        overdueEmiCount,
        dpd,
        priority,
        riskLevel,
        branch,
        assignedDate,
        status,
      ];
}

class RecoveryTimelineEvent extends Equatable {
  const RecoveryTimelineEvent({required this.title, required this.timestamp, required this.description});

  final String title;
  final DateTime timestamp;
  final String description;

  @override
  List<Object?> get props => [title, timestamp, description];
}

class EmiHistoryEntry extends Equatable {
  const EmiHistoryEntry({required this.month, required this.dueDate, required this.amount, required this.paid, this.paidDate});

  final String month;
  final DateTime dueDate;
  final double amount;
  final bool paid;
  final DateTime? paidDate;

  @override
  List<Object?> get props => [month, dueDate, amount, paid, paidDate];
}

enum PromiseStatus { pending, kept, broken }

extension PromiseStatusX on PromiseStatus {
  String get label => switch (this) {
        PromiseStatus.pending => 'Pending',
        PromiseStatus.kept => 'Kept',
        PromiseStatus.broken => 'Broken',
      };
}

class PromiseToPay extends Equatable {
  const PromiseToPay({
    required this.promisedAmount,
    required this.promisedDate,
    required this.remarks,
    required this.reminderDate,
    required this.status,
  });

  final double promisedAmount;
  final DateTime promisedDate;
  final String remarks;
  final DateTime reminderDate;
  final PromiseStatus status;

  @override
  List<Object?> get props => [promisedAmount, promisedDate, remarks, reminderDate, status];
}

enum PaymentMode { cash, upi, cheque, bankTransfer }

extension PaymentModeX on PaymentMode {
  String get label => switch (this) {
        PaymentMode.cash => 'Cash',
        PaymentMode.upi => 'UPI',
        PaymentMode.cheque => 'Cheque',
        PaymentMode.bankTransfer => 'Bank Transfer',
      };
}

class PaymentRecord extends Equatable {
  const PaymentRecord({
    required this.amount,
    required this.mode,
    required this.receiptNumber,
    required this.remarks,
    required this.recordedAt,
  });

  final double amount;
  final PaymentMode mode;
  final String receiptNumber;
  final String remarks;
  final DateTime recordedAt;

  @override
  List<Object?> get props => [amount, mode, receiptNumber, remarks, recordedAt];
}

class VisitRecord extends Equatable {
  const VisitRecord({
    required this.checkInTime,
    required this.latitude,
    required this.longitude,
    required this.customerPresent,
    required this.notes,
    required this.photoPaths,
    required this.documentPaths,
    this.signaturePath,
    this.pdfPath,
  });

  final DateTime checkInTime;
  final double latitude;
  final double longitude;
  final bool customerPresent;
  final String notes;
  final List<String> photoPaths;
  final List<String> documentPaths;
  final String? signaturePath;
  final String? pdfPath;

  VisitRecord copyWith({String? pdfPath}) => VisitRecord(
        checkInTime: checkInTime,
        latitude: latitude,
        longitude: longitude,
        customerPresent: customerPresent,
        notes: notes,
        photoPaths: photoPaths,
        documentPaths: documentPaths,
        signaturePath: signaturePath,
        pdfPath: pdfPath ?? this.pdfPath,
      );

  @override
  List<Object?> get props =>
      [checkInTime, latitude, longitude, customerPresent, notes, photoPaths, documentPaths, signaturePath, pdfPath];
}

class RecoveryReport extends Equatable {
  const RecoveryReport({
    required this.status,
    required this.remarks,
    required this.riskAssessment,
    required this.recommendation,
    required this.nextAction,
    required this.priority,
    required this.photoPaths,
    required this.documentPaths,
    required this.submittedAt,
    this.pdfPath,
  });

  final RecoveryStatus status;
  final String remarks;
  final RiskLevel riskAssessment;
  final String recommendation;
  final DateTime nextAction;
  final RecoveryPriority priority;
  final List<String> photoPaths;
  final List<String> documentPaths;
  final DateTime submittedAt;
  final String? pdfPath;

  RecoveryReport copyWith({String? pdfPath}) => RecoveryReport(
        status: status,
        remarks: remarks,
        riskAssessment: riskAssessment,
        recommendation: recommendation,
        nextAction: nextAction,
        priority: priority,
        photoPaths: photoPaths,
        documentPaths: documentPaths,
        submittedAt: submittedAt,
        pdfPath: pdfPath ?? this.pdfPath,
      );

  @override
  List<Object?> get props => [
        status,
        remarks,
        riskAssessment,
        recommendation,
        nextAction,
        priority,
        photoPaths,
        documentPaths,
        submittedAt,
        pdfPath,
      ];
}

class RecoveryDashboardSummary extends Equatable {
  const RecoveryDashboardSummary({
    required this.todaysAssignedCount,
    required this.pendingVisitsCount,
    required this.overdueAmount,
    required this.promiseToPayCount,
    required this.brokenPromiseCount,
    required this.closedCasesCount,
    required this.recoveryPerformancePercent,
    required this.collectionPercent,
  });

  final int todaysAssignedCount;
  final int pendingVisitsCount;
  final double overdueAmount;
  final int promiseToPayCount;
  final int brokenPromiseCount;
  final int closedCasesCount;
  final double recoveryPerformancePercent;
  final double collectionPercent;

  @override
  List<Object?> get props => [
        todaysAssignedCount,
        pendingVisitsCount,
        overdueAmount,
        promiseToPayCount,
        brokenPromiseCount,
        closedCasesCount,
        recoveryPerformancePercent,
        collectionPercent,
      ];
}
