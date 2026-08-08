import 'package:equatable/equatable.dart';

enum LoanType { personal, gold, home, business, vehicle, education, msme }

enum CasePriority { high, medium, low }

enum CaseStatus { assigned, inProgress, pendingReview, completed, rejected }

extension LoanTypeX on LoanType {
  String get label => switch (this) {
        LoanType.personal => 'Personal Loan',
        LoanType.gold => 'Gold Loan',
        LoanType.home => 'Home Loan',
        LoanType.business => 'Business Loan',
        LoanType.vehicle => 'Vehicle Loan',
        LoanType.education => 'Education Loan',
        LoanType.msme => 'MSME Loan',
      };

  /// Mock workflow rules — which verification steps apply to this loan type.
  List<String> get verificationSteps => switch (this) {
        LoanType.personal => const ['Identity Verification', 'Address Verification', 'Income Verification', 'Video KYC'],
        LoanType.gold => const ['Identity Verification', 'Gold Appraisal', 'Video KYC'],
        LoanType.home => const ['Identity Verification', 'Property Verification', 'Income Verification', 'Legal Document Check'],
        LoanType.business => const ['Identity Verification', 'Business Verification', 'Income Verification', 'Bank Statement Check'],
        LoanType.vehicle => const ['Identity Verification', 'Address Verification', 'Income Verification'],
        LoanType.education => const ['Identity Verification', 'Address Verification', 'Institution Verification'],
        LoanType.msme => const ['Identity Verification', 'Business Verification', 'Property Verification', 'Bank Statement Check'],
      };

  /// Documents an officer must physically capture during the visit for
  /// this loan type.
  List<String> get documentChecklist => switch (this) {
        LoanType.personal => const ['Aadhaar Card', 'PAN Card', 'Salary Slip', 'Bank Statement'],
        LoanType.gold => const ['Aadhaar Card', 'PAN Card', 'Gold Purity Certificate'],
        LoanType.home => const ['Aadhaar Card', 'PAN Card', 'Property Documents', 'Bank Statement'],
        LoanType.business => const ['Aadhaar Card', 'PAN Card', 'Business Registration', 'Bank Statement'],
        LoanType.vehicle => const ['Aadhaar Card', 'PAN Card', 'Salary Slip'],
        LoanType.education => const ['Aadhaar Card', 'PAN Card', 'Admission Letter'],
        LoanType.msme => const ['Aadhaar Card', 'PAN Card', 'Business Registration', 'Property Documents', 'Bank Statement'],
      };
}

extension CasePriorityX on CasePriority {
  String get label => switch (this) { CasePriority.high => 'High', CasePriority.medium => 'Medium', CasePriority.low => 'Low' };
}

extension CaseStatusX on CaseStatus {
  String get label => switch (this) {
        CaseStatus.assigned => 'Assigned',
        CaseStatus.inProgress => 'In Progress',
        CaseStatus.pendingReview => 'Pending Review',
        CaseStatus.completed => 'Completed',
        CaseStatus.rejected => 'Rejected',
      };
}

class CaseEntity extends Equatable {
  const CaseEntity({
    required this.id,
    required this.caseNumber,
    required this.customerName,
    required this.customerPhone,
    required this.address,
    required this.loanType,
    required this.loanAmount,
    required this.priority,
    required this.status,
    required this.assignedDate,
    required this.dueDate,
    this.cibilScore,
    this.references = const [],
  });

  /// Backend `LoanApplication.ID` (see internal/models/loan.go), as a
  /// string — this is what's sent to `POST /employee/loans/:id/verify`.
  final String id;
  final String caseNumber;
  final String customerName;
  final String customerPhone;
  final String address;
  final LoanType loanType;
  final double loanAmount;
  final CasePriority priority;
  final CaseStatus status;
  final DateTime assignedDate;
  final DateTime dueDate;

  /// Customer's CIBIL score (internal/loans/reference.go —
  /// `getOrCreateCibilScore`, 550-850), null if the backend omitted it.
  final int? cibilScore;

  /// The 2 mandatory loan references (internal/loans/reference.go —
  /// `MaskedReference`), always masked for Aadhaar/PAN.
  final List<LoanReference> references;

  @override
  List<Object?> get props => [
        id,
        caseNumber,
        customerName,
        customerPhone,
        address,
        loanType,
        loanAmount,
        priority,
        status,
        assignedDate,
        dueDate,
        cibilScore,
        references,
      ];
}

/// A loan reference contact, as returned by the backend's
/// `MaskedReference` (internal/loans/reference.go) — Aadhaar/PAN are
/// always masked server-side, raw values are never sent to this app.
class LoanReference extends Equatable {
  const LoanReference({
    required this.id,
    required this.name,
    required this.relation,
    required this.phone,
    required this.aadhaarMasked,
    required this.panMasked,
  });

  final String id;
  final String name;
  final String relation;
  final String phone;
  final String aadhaarMasked;
  final String panMasked;

  @override
  List<Object?> get props => [id, name, relation, phone, aadhaarMasked, panMasked];
}

/// Maps the backend's free-text loan `category` (see LoanApplication.Category
/// in internal/models/loan.go, and how it's set at submission time) onto the
/// app's closed [LoanType] enum for display/checklist purposes.
LoanType loanTypeFromBackendCategory(String category) {
  final normalized = category.toLowerCase();
  if (normalized.contains('gold')) return LoanType.gold;
  if (normalized.contains('home')) return LoanType.home;
  if (normalized.contains('business')) return LoanType.business;
  if (normalized.contains('vehicle') || normalized.contains('auto')) return LoanType.vehicle;
  if (normalized.contains('education')) return LoanType.education;
  if (normalized.contains('msme')) return LoanType.msme;
  return LoanType.personal;
}

/// Maps the backend's loan `status` string (internal/models/loan.go —
/// `assigned`/`verified`/...) onto the app's [CaseStatus] enum. Only
/// `assigned` and `verified` are reachable via the employee endpoints this
/// app calls, but the mapping is total for safety.
CaseStatus caseStatusFromBackend(String status) => switch (status) {
      'verified' || 'sanctioned' || 'disbursed' => CaseStatus.completed,
      'rejected' => CaseStatus.rejected,
      _ => CaseStatus.assigned,
    };

class CaseTimelineEvent extends Equatable {
  const CaseTimelineEvent({required this.title, required this.timestamp, required this.description});

  final String title;
  final DateTime timestamp;
  final String description;

  @override
  List<Object?> get props => [title, timestamp, description];
}
